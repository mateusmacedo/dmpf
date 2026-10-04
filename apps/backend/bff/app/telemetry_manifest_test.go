package app_test

import (
	"context"
	"errors"
	"maps"
	"net"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

const manifestPath = "../deploy/.env.example"

var legacyVariables = []string{"OTLP_ENDPOINT", "OTLP_INSECURE", "OTLP_LOGS", "SERVICE", "SERVICE_VERSION", "INSTANCE_ID", "TRACE_SAMPLE_RATE"}

type otlpExports struct {
	mu        sync.Mutex
	resources map[string]*resourcepb.Resource
	logs      []*logspb.LogRecord
}

func (e *otlpExports) record(signal string, resource *resourcepb.Resource) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.resources[signal] = resource
}

func (e *otlpExports) keep(records []*logspb.LogRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logs = append(e.logs, records...)
}

func (e *otlpExports) logRecords() []*logspb.LogRecord {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]*logspb.LogRecord(nil), e.logs...)
}

func (e *otlpExports) identity(signal string) (map[string]string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	resource, ok := e.resources[signal]
	if !ok {
		return nil, false
	}
	identity := map[string]string{}
	for _, attribute := range resource.GetAttributes() {
		identity[attribute.GetKey()] = attribute.GetValue().GetStringValue()
	}
	return identity, true
}

type otlpTraceSink struct {
	coltracepb.UnimplementedTraceServiceServer
	got *otlpExports
}

func (s otlpTraceSink) Export(_ context.Context, request *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	for _, spans := range request.GetResourceSpans() {
		s.got.record("traces", spans.GetResource())
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

type otlpMetricSink struct {
	colmetricpb.UnimplementedMetricsServiceServer
	got *otlpExports
}

func (s otlpMetricSink) Export(_ context.Context, request *colmetricpb.ExportMetricsServiceRequest) (*colmetricpb.ExportMetricsServiceResponse, error) {
	for _, metrics := range request.GetResourceMetrics() {
		s.got.record("metrics", metrics.GetResource())
	}
	return &colmetricpb.ExportMetricsServiceResponse{}, nil
}

type otlpLogSink struct {
	collogspb.UnimplementedLogsServiceServer
	got *otlpExports
}

func (s otlpLogSink) Export(_ context.Context, request *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	for _, logs := range request.GetResourceLogs() {
		s.got.record("logs", logs.GetResource())
		for _, scope := range logs.GetScopeLogs() {
			s.got.keep(scope.GetLogRecords())
		}
	}
	return &collogspb.ExportLogsServiceResponse{}, nil
}

func startCollector(t *testing.T) (string, *otlpExports) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	got := &otlpExports{resources: map[string]*resourcepb.Resource{}}
	server := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(server, otlpTraceSink{got: got})
	colmetricpb.RegisterMetricsServiceServer(server, otlpMetricSink{got: got})
	collogspb.RegisterLogsServiceServer(server, otlpLogSink{got: got})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return listener.Addr().String(), got
}

func readManifest(t *testing.T) map[string]string {
	t.Helper()
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read %s: %v", manifestPath, err)
	}
	values := map[string]string{}
	for line := range strings.Lines(string(content)) {
		line = strings.TrimSpace(line)
		if key, value, ok := strings.Cut(line, "="); ok && !strings.HasPrefix(line, "#") {
			values[key] = value
		}
	}
	return values
}

func useOnlyTheOTelEnvironmentOf(t *testing.T, env map[string]string) {
	t.Helper()
	for _, pair := range os.Environ() {
		if key, _, _ := strings.Cut(pair, "="); strings.HasPrefix(key, "OTEL_") {
			t.Setenv(key, "")
			_ = os.Unsetenv(key)
		}
	}
	for key, value := range env {
		if strings.HasPrefix(key, "OTEL_") {
			t.Setenv(key, value)
		}
	}
}

func TestTheManifestDeclaresTheCanonicalOTelEnvironment(t *testing.T) {
	manifest := readManifest(t)

	for key, want := range map[string]string{
		"OTEL_SERVICE_NAME":           "bff",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_TRACES_SAMPLER_ARG":     "1.0",
		"OTEL_LOGS_EXPORTER":          "otlp",
		"OTEL_PROPAGATORS":            "tracecontext",
		"OTEL_GO_X_OBSERVABILITY":     "true",
	} {
		if got := manifest[key]; got != want {
			t.Errorf("%s = %q, want %q (RF-E1, RF-E5, RF-D7)", key, got, want)
		}
	}
	if endpoint, err := url.Parse(manifest["OTEL_EXPORTER_OTLP_ENDPOINT"]); err != nil || endpoint.Scheme != "http" || endpoint.Port() != "4317" {
		t.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT = %q, want http:// on the gRPC port 4317 of the Collector (RF-E1)", manifest["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}
	for _, attribute := range []string{"service.version=", "service.instance.id=", "deployment.environment.name="} {
		if !strings.Contains(manifest["OTEL_RESOURCE_ATTRIBUTES"], attribute) {
			t.Errorf("OTEL_RESOURCE_ATTRIBUTES = %q, want %s (RF-E3)", manifest["OTEL_RESOURCE_ATTRIBUTES"], attribute)
		}
	}
	for _, legacy := range legacyVariables {
		if _, declared := manifest[legacy]; declared {
			t.Errorf("%s is declared, want it gone in favor of OTEL_* (RF-E1)", legacy)
		}
	}
}

func TestTheManifestEnvironmentExportsEverySignalOverGRPC(t *testing.T) {
	for _, role := range []string{"api"} {
		t.Run(role, func(t *testing.T) {
			addr, collector := startCollector(t)
			env := maps.Clone(readManifest(t))
			env["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://" + addr
			useOnlyTheOTelEnvironmentOf(t, env)
			cfg, err := app.FromEnv(func(key string) string { return env[key] })
			if err != nil {
				t.Fatalf("FromEnv() over %s = %v", manifestPath, err)
			}

			ctx := context.Background()
			runtime, err := boot.StartTelemetry(ctx, app.TelemetryOf(cfg))
			if err != nil {
				t.Fatalf("StartTelemetry() = %v", err)
			}
			_, span := runtime.Tracer().Start(ctx, "manifest.probe")
			span.End()
			runtime.LoggerFor(reflect.TypeFor[app.Config]().PkgPath()).InfoContext(ctx, "manifest probe")
			grace, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			if err := runtime.Shutdown(grace); err != nil {
				t.Fatalf("Shutdown() = %v", err)
			}

			for _, signal := range []string{"traces", "metrics", "logs"} {
				identity, ok := collector.identity(signal)
				if !ok {
					t.Errorf("the OTLP/gRPC receiver got no %s (RF-E1)", signal)
					continue
				}
				if identity["service.name"] != "bff" || identity["dmpf.process.role"] != role {
					t.Errorf("%s resource = %v, want service.name=bff and dmpf.process.role=%s (RF-E3)", signal, identity, role)
				}
			}
		})
	}
}

func TestAStartWithoutTheVersionOrTheInstanceInTheEnvironmentIsRefused(t *testing.T) {
	for _, missing := range []string{"service.version", "service.instance.id"} {
		t.Run(missing, func(t *testing.T) {
			addr, _ := startCollector(t)
			env := maps.Clone(readManifest(t))
			env["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://" + addr
			var kept []string
			for _, pair := range strings.Split(env["OTEL_RESOURCE_ATTRIBUTES"], ",") {
				if !strings.HasPrefix(pair, missing+"=") {
					kept = append(kept, pair)
				}
			}
			env["OTEL_RESOURCE_ATTRIBUTES"] = strings.Join(kept, ",")
			useOnlyTheOTelEnvironmentOf(t, env)
			cfg, err := app.FromEnv(func(key string) string { return env[key] })
			if err != nil {
				t.Fatalf("FromEnv() over %s = %v", manifestPath, err)
			}

			runtime, err := boot.StartTelemetry(context.Background(), app.TelemetryOf(cfg))
			if err == nil {
				_ = runtime.Shutdown(context.Background())
			}

			if !errors.Is(err, otelboot.ErrResourceIncomplete) {
				t.Fatalf("StartTelemetry() without %s = %v, want %v (RF-E3)", missing, err, otelboot.ErrResourceIncomplete)
			}
		})
	}
}
