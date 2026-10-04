package boot_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/grpc"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

const (
	emitterScope = "github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	libraryScope = "github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
)

var otelVariables = []string{
	"OTEL_SDK_DISABLED",
	"OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER", "OTEL_LOGS_EXPORTER",
	"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_PROTOCOL", "OTEL_EXPORTER_OTLP_INSECURE",
	"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
	"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL", "OTEL_EXPORTER_OTLP_LOGS_PROTOCOL",
	"OTEL_METRICS_PRODUCERS", "OTEL_TRACES_SAMPLER", "OTEL_TRACES_SAMPLER_ARG",
	"OTEL_SERVICE_NAME", "OTEL_RESOURCE_ATTRIBUTES", "OTEL_PROPAGATORS",
}

func cleanOTelEnv(t *testing.T) {
	t.Helper()
	for _, variable := range otelVariables {
		t.Setenv(variable, "")
		_ = os.Unsetenv(variable)
	}
}

func exportNothing(t *testing.T) {
	t.Helper()
	cleanOTelEnv(t)
	for _, variable := range []string{"OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER", "OTEL_LOGS_EXPORTER"} {
		t.Setenv(variable, "none")
	}
}

type traceReceiver struct {
	coltracepb.UnimplementedTraceServiceServer
	calls    atomic.Int64
	resource sync.Map
}

func (r *traceReceiver) Export(_ context.Context, request *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	r.calls.Add(1)
	for _, spans := range request.GetResourceSpans() {
		for _, attribute := range spans.GetResource().GetAttributes() {
			r.resource.Store(attribute.GetKey(), attribute.GetValue().GetStringValue())
		}
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

type metricReceiver struct {
	colmetricpb.UnimplementedMetricsServiceServer
	calls atomic.Int64
	names sync.Map
}

func (r *metricReceiver) Export(_ context.Context, request *colmetricpb.ExportMetricsServiceRequest) (*colmetricpb.ExportMetricsServiceResponse, error) {
	r.calls.Add(1)
	for _, resource := range request.GetResourceMetrics() {
		for _, scope := range resource.GetScopeMetrics() {
			for _, m := range scope.GetMetrics() {
				r.names.Store(m.GetName(), true)
			}
		}
	}
	return &colmetricpb.ExportMetricsServiceResponse{}, nil
}

type logReceiver struct {
	collogspb.UnimplementedLogsServiceServer
	calls   atomic.Int64
	mu      sync.Mutex
	records []exportedLog
}

type exportedLog struct {
	scope      string
	body       string
	severity   string
	at         uint64
	observed   uint64
	attributes map[string]string
	values     map[string]*commonpb.AnyValue
	resource   map[string]string
}

func (r *logReceiver) Export(_ context.Context, request *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	r.calls.Add(1)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, resource := range request.GetResourceLogs() {
		identity := map[string]string{}
		for _, attribute := range resource.GetResource().GetAttributes() {
			identity[attribute.GetKey()] = attribute.GetValue().GetStringValue()
		}
		for _, scope := range resource.GetScopeLogs() {
			for _, record := range scope.GetLogRecords() {
				attributes := make(map[string]string, len(record.GetAttributes()))
				values := make(map[string]*commonpb.AnyValue, len(record.GetAttributes()))
				for _, attribute := range record.GetAttributes() {
					value := attribute.GetValue()
					values[attribute.GetKey()] = value
					attributes[attribute.GetKey()] = value.GetStringValue()
					if value.GetStringValue() == "" {
						attributes[attribute.GetKey()] = value.String()
					}
				}
				r.records = append(r.records, exportedLog{
					scope: scope.GetScope().GetName(), body: record.GetBody().GetStringValue(),
					severity: record.GetSeverityText(), at: record.GetTimeUnixNano(), observed: record.GetObservedTimeUnixNano(),
					attributes: attributes, values: values, resource: identity,
				})
			}
		}
	}
	return &collogspb.ExportLogsServiceResponse{}, nil
}

func (r *logReceiver) exported() []exportedLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]exportedLog(nil), r.records...)
}

type receiver struct {
	addr    string
	traces  traceReceiver
	metrics metricReceiver
	logs    logReceiver
}

func startReceiver(t *testing.T) *receiver {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	r := &receiver{addr: listener.Addr().String()}
	server := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(server, &r.traces)
	colmetricpb.RegisterMetricsServiceServer(server, &r.metrics)
	collogspb.RegisterLogsServiceServer(server, &r.logs)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return r
}

func exportLogsOnly(t *testing.T) *receiver {
	t.Helper()
	cleanOTelEnv(t)
	collector := startReceiver(t)
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+collector.addr)
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	return collector
}

type countingListener struct {
	net.Listener
	accepted atomic.Int64
}

func startCountingListener(t *testing.T) *countingListener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	counting := &countingListener{Listener: listener}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			counting.accepted.Add(1)
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })
	return counting
}

func sampledTelemetry() boot.Telemetry {
	telemetry := telemetryUnderTest()
	telemetry.Signals.Sampling = tracing.UniformRates(1)
	return telemetry
}

func emitAndShutdown(t *testing.T, runtime *otelboot.Runtime) {
	t.Helper()
	ctx := context.Background()
	_, span := runtime.Tracer().Start(ctx, "orders.place")
	span.End()
	runtime.Instruments().Retries.Add(ctx, 1)
	runtime.LoggerFor(emitterScope).InfoContext(ctx, "placed")

	grace, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := runtime.Shutdown(grace); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
}

func TestExportersDeclaredNoneOpenNoConnection(t *testing.T) {
	cleanOTelEnv(t)
	listener := startCountingListener(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+listener.Addr().String())
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	for _, variable := range []string{"OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER", "OTEL_LOGS_EXPORTER"} {
		t.Setenv(variable, "none")
	}

	runtime, err := boot.StartTelemetry(context.Background(), sampledTelemetry())
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	emitAndShutdown(t, runtime)

	if got := listener.accepted.Load(); got != 0 {
		t.Fatalf("the collector accepted %d connections, want none with every exporter declared none", got)
	}
}

func TestWithoutALogPipelineTheRecordsAreDiscarded(t *testing.T) {
	for name, declared := range map[string]map[string]string{
		"logs exporter none": {"OTEL_TRACES_EXPORTER": "none", "OTEL_METRICS_EXPORTER": "none", "OTEL_LOGS_EXPORTER": "none"},
		"sdk disabled":       {"OTEL_SDK_DISABLED": "true"},
	} {
		t.Run(name, func(t *testing.T) {
			cleanOTelEnv(t)
			for variable, value := range declared {
				t.Setenv(variable, value)
			}
			telemetry := sampledTelemetry()
			telemetry.Signals.IgnoredSampler = "always_on"

			runtime, err := boot.StartTelemetry(context.Background(), telemetry)
			if err != nil {
				t.Fatalf("StartTelemetry() = %v", err)
			}
			library := logging.NewLogger(runtime.LoggerProvider(), libraryScope)
			enabled := runtime.LoggerFor(emitterScope).Enabled(context.Background(), slog.LevelError) ||
				library.Enabled(context.Background(), slog.LevelError)
			runtime.LoggerFor(emitterScope).ErrorContext(context.Background(), "payment failed")
			library.ErrorContext(context.Background(), "payment failed")
			otel.Handle(errors.New("traces export: connection refused"))
			if err := runtime.Shutdown(context.Background()); err != nil {
				t.Fatalf("Shutdown() = %v", err)
			}

			if enabled {
				t.Fatal("a logger is enabled, want every record discarded without a log pipeline (RF-A1)")
			}
		})
	}
}

func refusedEndpoint(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}

func stalledEndpoint(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, conn)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range held {
			_ = conn.Close()
		}
	})
	return listener.Addr().String()
}

func returnsWithin(t *testing.T, limit time.Duration, emitter string, run func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		run()
	}()
	select {
	case <-done:
	case <-time.After(limit):
		t.Errorf("the %s did not return in %v with the export failing, want the SDK to drop instead of blocking (NF Degradação)", emitter, limit)
	}
}

func repeated(emit func()) func() {
	const count, window = 10_000, 200 * time.Millisecond
	return func() {
		started := time.Now()
		for i := 0; i < count || time.Since(started) < window; i++ {
			emit()
		}
	}
}

func TestAFailingExportNeverBlocksTheLogQueueNorTheMetricReader(t *testing.T) {
	for name, endpoint := range map[string]func(*testing.T) string{
		"unreachable endpoint": refusedEndpoint,
		"stalled collector":    stalledEndpoint,
	} {
		t.Run(name, func(t *testing.T) {
			cleanOTelEnv(t)
			t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+endpoint(t))
			t.Setenv("OTEL_TRACES_EXPORTER", "none")
			t.Setenv("OTEL_METRICS_EXPORTER", "otlp")
			t.Setenv("OTEL_LOGS_EXPORTER", "otlp")
			t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "10")

			runtime, err := boot.StartTelemetry(context.Background(), sampledTelemetry())
			if err != nil {
				t.Fatalf("StartTelemetry() = %v", err)
			}
			t.Cleanup(func() {
				grace, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				returnsWithin(t, 3*time.Second, "shutdown", func() { _ = runtime.Shutdown(grace) })
			})
			ctx := context.Background()

			logger := runtime.LoggerFor(emitterScope)
			returnsWithin(t, 5*time.Second, "log emitter", repeated(func() { logger.InfoContext(ctx, "order placed") }))
			returnsWithin(t, 5*time.Second, "metric recorder", repeated(func() { runtime.Instruments().Retries.Add(ctx, 1) }))
		})
	}
}

func TestOTLPOverGRPCReachesTheDeclaredEndpointInThePlainOnEverySignal(t *testing.T) {
	cleanOTelEnv(t)
	collector := startReceiver(t)
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+collector.addr)
	for _, variable := range []string{"OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER", "OTEL_LOGS_EXPORTER"} {
		t.Setenv(variable, "otlp")
	}

	runtime, err := boot.StartTelemetry(context.Background(), sampledTelemetry())
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	emitAndShutdown(t, runtime)

	if collector.traces.calls.Load() == 0 || collector.metrics.calls.Load() == 0 || collector.logs.calls.Load() == 0 {
		t.Fatalf("the gRPC receiver got traces=%d metrics=%d logs=%d, want every signal",
			collector.traces.calls.Load(), collector.metrics.calls.Load(), collector.logs.calls.Load())
	}
}

func TestWithoutADeclaredProtocolTheExportIsHTTPProtobuf(t *testing.T) {
	cleanOTelEnv(t)
	var paths sync.Map
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths.Store(r.Method+" "+r.URL.Path+" "+r.Header.Get("Content-Type"), true)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", server.URL)
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")

	runtime, err := boot.StartTelemetry(context.Background(), sampledTelemetry())
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	emitAndShutdown(t, runtime)

	if _, ok := paths.Load("POST /v1/traces application/x-protobuf"); !ok {
		var seen []string
		paths.Range(func(key, _ any) bool { seen = append(seen, key.(string)); return true })
		t.Fatalf("requests = %v, want POST /v1/traces in application/x-protobuf, the default of autoexport", seen)
	}
}

func TestTheRuntimeLogsLeaveByOTLPAtTheDeclaredLevel(t *testing.T) {
	cleanOTelEnv(t)
	collector := startReceiver(t)
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+collector.addr)
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	signals, err := boot.SignalsFromEnv(func(variable string) string {
		if variable == boot.EnvLogLevel {
			return "info"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("SignalsFromEnv() = %v", err)
	}
	telemetry := telemetryUnderTest()
	telemetry.Signals = signals

	runtime, err := boot.StartTelemetry(context.Background(), telemetry)
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	library := logging.NewLogger(runtime.LoggerProvider(), libraryScope)
	library.DebugContext(context.Background(), "detail")
	library.InfoContext(context.Background(), "order placed")
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	exported := collector.logs.exported()
	if len(exported) != 1 || exported[0].body != "order placed" {
		t.Fatalf("exported %+v, want the info record alone under LOG_LEVEL=info", exported)
	}
	if exported[0].scope == telemetry.Service {
		t.Fatalf("scope = %q, want an import path and not the service name (RF-A1)", exported[0].scope)
	}
}

func TestARecordLeavesWithItsObservedTimeUnderTheWholeResource(t *testing.T) {
	collector := exportLogsOnly(t)
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "deployment.environment.name=hmg")
	telemetry := sampledTelemetry()
	telemetry.Role = "api"

	runtime, err := boot.StartTelemetry(context.Background(), telemetry)
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	runtime.LoggerFor(emitterScope).InfoContext(context.Background(), "order placed")
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	placed := recordsNamed(collector.logs.exported(), "order placed")
	if len(placed) != 1 {
		t.Fatalf("exported %+v, want one \"order placed\"", collector.logs.exported())
	}
	if placed[0].observed == 0 {
		t.Error("ObservedTimestamp is unset, want the time the SDK observed the record (RF-A2)")
	}
	for key, want := range map[string]string{
		"service.name": "orders", "service.version": "1.2.3", "service.instance.id": "pod-1",
		"deployment.environment.name": "hmg", otelboot.ProcessRoleAttribute: "api",
	} {
		if got := placed[0].resource[key]; got != want {
			t.Errorf("resource %s = %q, want %q on the log record (RF-A2)", key, got, want)
		}
	}
}

func TestTheDeclaredLogLevelReachesEveryLoggerOfTheRuntime(t *testing.T) {
	cleanOTelEnv(t)
	collector := startReceiver(t)
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+collector.addr)
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	signals, err := boot.SignalsFromEnv(func(variable string) string {
		if variable == boot.EnvLogLevel {
			return "warn"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("SignalsFromEnv() = %v", err)
	}
	telemetry := telemetryUnderTest()
	telemetry.Signals = signals

	runtime, err := boot.StartTelemetry(context.Background(), telemetry)
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	library := logging.NewLogger(runtime.LoggerProvider(), libraryScope)
	library.InfoContext(context.Background(), "order placed")
	runtime.LoggerFor(emitterScope).InfoContext(context.Background(), "order placed")
	library.WarnContext(context.Background(), "order delayed")
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	exported := collector.logs.exported()
	if len(exported) != 1 || exported[0].body != "order delayed" {
		t.Fatalf("exported %+v, want the warn record alone under LOG_LEVEL=warn", exported)
	}
}

func TestADisabledSDKHandsANoopRuntime(t *testing.T) {
	cleanOTelEnv(t)
	collector := startReceiver(t)
	t.Setenv("OTEL_SDK_DISABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+collector.addr)
	for _, variable := range []string{"OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER", "OTEL_LOGS_EXPORTER"} {
		t.Setenv(variable, "otlp")
	}

	runtime, err := boot.StartTelemetry(context.Background(), sampledTelemetry())
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	_, span := runtime.Tracer().Start(context.Background(), "orders.place")
	recording := span.IsRecording()
	span.End()
	emitAndShutdown(t, runtime)

	if recording {
		t.Error("the span records under OTEL_SDK_DISABLED=true, want a noop tracer")
	}
	if got := collector.traces.calls.Load() + collector.metrics.calls.Load() + collector.logs.calls.Load(); got != 0 {
		t.Fatalf("the collector got %d exports under OTEL_SDK_DISABLED=true, want none", got)
	}
}

func TestTheDeclaredRoleOfTheProcessIsOnTheResource(t *testing.T) {
	cleanOTelEnv(t)
	collector := startReceiver(t)
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+collector.addr)
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")

	telemetry := sampledTelemetry()
	telemetry.Role = "consumer"
	runtime, err := boot.StartTelemetry(context.Background(), telemetry)
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	emitAndShutdown(t, runtime)

	if role, _ := collector.traces.resource.Load(otelboot.ProcessRoleAttribute); role != "consumer" {
		t.Fatalf("%s = %v, want consumer", otelboot.ProcessRoleAttribute, role)
	}
}

func TestADeclaredSamplerIsWarnedAboutOnce(t *testing.T) {
	collector := exportLogsOnly(t)
	telemetry := sampledTelemetry()
	telemetry.Signals.IgnoredSampler = "always_on"

	runtime, err := boot.StartTelemetry(context.Background(), telemetry)
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	emitAndShutdown(t, runtime)

	got, severity := 0, ""
	for _, record := range collector.logs.exported() {
		mentions := strings.Count(record.body, "OTEL_TRACES_SAMPLER")
		for _, value := range record.attributes {
			mentions += strings.Count(value, "OTEL_TRACES_SAMPLER")
		}
		if mentions > 0 {
			severity = record.severity
		}
		got += mentions
	}
	if got != 1 || severity != "WARN" {
		t.Fatalf("exported records name OTEL_TRACES_SAMPLER %d times, the last at %q, want one WARN record (RF-E5):\n%+v",
			got, severity, collector.logs.exported())
	}
}

func TestADeclaredSamplerInTheEnvironmentIsWarnedAboutOnce(t *testing.T) {
	for name, declared := range map[string]string{"valid": "always_on", "invalid": "bogus", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			collector := exportLogsOnly(t)
			t.Setenv(boot.EnvTracesSampler, declared)
			signals, err := boot.SignalsFromEnv(os.Getenv)
			if err != nil {
				t.Fatalf("SignalsFromEnv() = %v", err)
			}
			telemetry := sampledTelemetry()
			telemetry.Signals = signals

			runtime, err := boot.StartTelemetry(context.Background(), telemetry)
			if err != nil {
				t.Fatalf("StartTelemetry() = %v", err)
			}
			if err := runtime.Shutdown(context.Background()); err != nil {
				t.Fatalf("Shutdown() = %v", err)
			}

			var warnings []exportedLog
			for _, record := range collector.logs.exported() {
				if record.severity == "WARN" || strings.Contains(record.body, boot.EnvTracesSampler) {
					warnings = append(warnings, record)
				}
			}
			if len(warnings) != 1 || warnings[0].severity != "WARN" || !strings.HasPrefix(warnings[0].body, boot.EnvTracesSampler) {
				t.Fatalf("%s=%q exported the warnings %+v, want one WARN record, naming the ignored variable (RF-E5)",
					boot.EnvTracesSampler, declared, warnings)
			}
		})
	}
}

func TestTheWarnOfAnIgnoredSamplerCarriesTheDeclaredValuePastTheAllowlist(t *testing.T) {
	const key = "dmpf.sampler.declared"
	fromEnvironment := func(declared string) func(*testing.T, *boot.Telemetry) {
		return func(t *testing.T, telemetry *boot.Telemetry) {
			t.Setenv(boot.EnvTracesSampler, declared)
			signals, err := boot.SignalsFromEnv(os.Getenv)
			if err != nil {
				t.Fatalf("SignalsFromEnv() = %v", err)
			}
			telemetry.Signals = signals
		}
	}
	cases := map[string]struct {
		declared string
		declare  func(*testing.T, *boot.Telemetry)
	}{
		"by the signals": {"always_on", func(_ *testing.T, telemetry *boot.Telemetry) {
			telemetry.Signals.IgnoredSampler = "always_on"
		}},
		"valid in the environment":   {"always_on", fromEnvironment("always_on")},
		"invalid in the environment": {"bogus", fromEnvironment("bogus")},
		"empty in the environment":   {"", fromEnvironment("")},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			collector := exportLogsOnly(t)
			telemetry := sampledTelemetry()
			tc.declare(t, &telemetry)

			runtime, err := boot.StartTelemetry(context.Background(), telemetry)
			if err != nil {
				t.Fatalf("StartTelemetry() = %v", err)
			}
			if err := runtime.Shutdown(context.Background()); err != nil {
				t.Fatalf("Shutdown() = %v", err)
			}

			var warned []exportedLog
			for _, record := range collector.logs.exported() {
				if strings.HasPrefix(record.body, boot.EnvTracesSampler) {
					warned = append(warned, record)
				}
			}
			if len(warned) != 1 {
				t.Fatalf("exported %d warnings naming %s, want 1: %+v", len(warned), boot.EnvTracesSampler, warned)
			}
			value, carried := warned[0].values[key]
			if !carried || value.GetStringValue() != tc.declared {
				t.Fatalf("the warning carries %v, want %s=%q past the allowlist of the processor (RF-A6)",
					warned[0].attributes, key, tc.declared)
			}
		})
	}
}

func TestTheLogIsSampledByTheTableOfTRC13NotByTheSamplerArgument(t *testing.T) {
	cases := []struct {
		class       tracing.Class
		emitted     int
		least, most int
		want        string
	}{
		{tracing.ClassWrite, 1000, 1000, 1000, "every record at the write rate of 1.0"},
		{tracing.ClassRead, 10000, 40, 200, "the read rate of 0.01 (40..200)"},
	}
	for _, tc := range cases {
		t.Run(string(tc.class), func(t *testing.T) {
			collector := exportLogsOnly(t)
			t.Setenv(boot.EnvTracesSamplerArg, "0")
			signals, err := boot.SignalsFromEnv(os.Getenv)
			if err != nil {
				t.Fatalf("SignalsFromEnv() = %v", err)
			}
			telemetry := telemetryUnderTest()
			telemetry.Class = tc.class
			telemetry.Signals = signals

			runtime, err := boot.StartTelemetry(context.Background(), telemetry)
			if err != nil {
				t.Fatalf("StartTelemetry() = %v", err)
			}
			ctx, span := runtime.Tracer().Start(context.Background(), "orders.place")
			if span.SpanContext().IsSampled() {
				t.Fatal("the span is sampled under OTEL_TRACES_SAMPLER_ARG=0, want a trace the head dropped")
			}
			for range tc.emitted {
				runtime.LoggerFor(emitterScope).InfoContext(ctx, "placed in an unsampled trace")
			}
			span.End()
			if err := runtime.Shutdown(context.Background()); err != nil {
				t.Fatalf("Shutdown() = %v", err)
			}

			if got := len(recordsNamed(collector.logs.exported(), "placed in an unsampled trace")); got < tc.least || got > tc.most {
				t.Fatalf("exported %d of %d records of a %s process, want %s of TRC-13, "+
					"not the 0 of OTEL_TRACES_SAMPLER_ARG (LOG-12)", got, tc.emitted, tc.class, tc.want)
			}
		})
	}
}

func TestTheGlobalPropagatorInjectsNoBaggage(t *testing.T) {
	exportNothing(t)
	runtime, err := boot.StartTelemetry(context.Background(), sampledTelemetry())
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	member, _ := baggage.NewMemberRaw(tracing.KeyTenantID, "tenant-1")
	bag, _ := baggage.New(member)
	ctx, span := runtime.Tracer().Start(baggage.ContextWithBaggage(context.Background(), bag), "orders.place")
	defer span.End()
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	if _, leaked := carrier["baggage"]; leaked || carrier["traceparent"] == "" {
		t.Fatalf("carrier = %v, want traceparent and no baggage", carrier)
	}
}

func TestADeclaredPropagatorOtherThanTraceContextRefusesTheStart(t *testing.T) {
	for _, declared := range []string{"b3", "baggage"} {
		t.Run(declared, func(t *testing.T) {
			cleanOTelEnv(t)
			t.Setenv("OTEL_PROPAGATORS", declared)

			if _, err := boot.StartTelemetry(context.Background(), sampledTelemetry()); !errors.Is(err, otelboot.ErrPropagatorNotW3C) {
				t.Fatalf("StartTelemetry() with OTEL_PROPAGATORS=%s = %v, want ErrPropagatorNotW3C (RF-E4)", declared, err)
			}
		})
	}
}

func TestAnSDKErrorIsLoggedWithoutItsMessage(t *testing.T) {
	collector := exportLogsOnly(t)
	runtime, err := boot.StartTelemetry(context.Background(), sampledTelemetry())
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}

	otel.Handle(errors.New("traces export: dial tcp 10.0.0.7:4317: connect: connection refused"))
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	typed, leaked := false, false
	for _, record := range collector.logs.exported() {
		leaked = leaked || strings.Contains(record.body, "10.0.0.7")
		for key, value := range record.attributes {
			typed = typed || key == "error.type"
			leaked = leaked || strings.Contains(value, "10.0.0.7")
		}
	}
	if leaked || !typed {
		t.Fatalf("exported %+v, want the export failure reduced to error.type, without the address (RF-A3)", collector.logs.exported())
	}
}

func TestTheMetricReaderOfAutoexportCarriesTheGoRuntime(t *testing.T) {
	cleanOTelEnv(t)
	collector := startReceiver(t)
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+collector.addr)
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "otlp")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")

	runtime, err := boot.StartTelemetry(context.Background(), sampledTelemetry())
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	emitAndShutdown(t, runtime)

	for _, name := range []string{"go.schedule.duration", "go.goroutine.count", "go.memory.used"} {
		if _, exported := collector.metrics.names.Load(name); !exported {
			t.Errorf("%s did not reach the collector: the runtime instrumentation and its producer are bound at boot (RF-D5)", name)
		}
	}
}

func TestAnHTTPEndpointWarnsOncePerProcessThatTheExportGoesInThePlain(t *testing.T) {
	boot.ResetPlaintextExportWarning()
	collector := exportLogsOnly(t)

	for range 2 {
		runtime, err := boot.StartTelemetry(context.Background(), telemetryUnderTest())
		if err != nil {
			t.Fatalf("StartTelemetry() = %v", err)
		}
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() = %v", err)
		}
	}

	var warnings []string
	for _, record := range collector.logs.exported() {
		if strings.Contains(record.body, "without TLS") {
			warnings = append(warnings, record.severity+" "+record.body)
		}
	}
	want := "WARN telemetry: OTLP export without TLS by explicit development-only opt-out (http:// endpoint)"
	if len(warnings) != 1 || warnings[0] != want {
		t.Fatalf("two starts over an http:// endpoint warned %q, want one per process %q (GRP-15)", warnings, want)
	}
}
