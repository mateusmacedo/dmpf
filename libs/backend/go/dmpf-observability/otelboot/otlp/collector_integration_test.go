package otlp_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/otelboot"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/otelboot/otlp"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
)

// collectorImage is pinned by digest, not by tag: a tag is mutable and a green
// suite that turns red because someone republished 0.160.0 would be a failure
// nobody could reproduce.
const collectorImage = "otel/opentelemetry-collector:0.160.0@sha256:" +
	"e495787f07dbe432ce763ebaf5bc3d113850e9eee2250ade7a3da6a882d0d69a"

// collectorConfig is the smallest pipeline that proves the handshake: OTLP over
// gRPC in, the debug exporter out, so what arrived shows up in the container
// logs and the test reads it back.
const collectorConfig = `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  debug:
    verbosity: detailed

service:
  telemetry:
    logs:
      level: info
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
    metrics:
      receivers: [otlp]
      exporters: [debug]
`

// The collector is started once for the package: both tests speak to the same
// plaintext receiver, and starting a container per test would double the only
// part of this suite that is not fast.
var collector struct {
	endpoint string
	logs     func() (string, error)
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := testcontainers.Run(ctx, collectorImage,
		testcontainers.WithExposedPorts("4317/tcp"),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			Reader:            strings.NewReader(collectorConfig),
			ContainerFilePath: "/etc/otelcol/config.yaml",
			FileMode:          0o644,
		}),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("4317/tcp")),
	)
	if err != nil {
		_ = testcontainers.TerminateContainer(container)
		fmt.Fprintf(os.Stderr, "Run(collector) = %v; the suite needs a reachable Docker daemon\n", err)
		os.Exit(1)
	}

	collector.endpoint, err = container.PortEndpoint(ctx, "4317", "")
	if err != nil {
		_ = testcontainers.TerminateContainer(container)
		fmt.Fprintf(os.Stderr, "PortEndpoint() = %v\n", err)
		os.Exit(1)
	}
	collector.logs = func() (string, error) {
		stream, err := container.Logs(ctx)
		if err != nil {
			return "", err
		}
		defer func() { _ = stream.Close() }()

		recorded, err := io.ReadAll(stream)
		return string(recorded), err
	}

	code := m.Run()
	if err := testcontainers.TerminateContainer(container); err != nil {
		fmt.Fprintf(os.Stderr, "TerminateContainer() = %v\n", err)
	}
	os.Exit(code)
}

func TestASpanAndAMetricReachTheCollector(t *testing.T) {
	ctx := context.Background()
	config := transportConfig(otelboot.Transport{Endpoint: collector.endpoint, Insecure: true}, true)

	exporter, err := otlp.TraceExporter(ctx, config)
	if err != nil {
		t.Fatalf("TraceExporter() = %v", err)
	}
	reader, err := otlp.MetricReader(ctx, config, sdkmetric.WithInterval(time.Hour))
	if err != nil {
		t.Fatalf("MetricReader() = %v", err)
	}

	config.TraceExporter = exporter
	config.MetricReader = reader
	config.Sampling = tracing.Rates{tracing.ClassMaintenance: 1.0}

	runtime, err := otelboot.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}

	_, span := runtime.Tracer().Start(ctx, "orders.place",
		trace.WithAttributes(tracing.Attributes{}.TrafficClass(string(tracing.ClassMaintenance)).KeyValues()...))
	span.End()
	runtime.Instruments().Requests.Add(ctx, 1)

	// Shutdown flushes both pipelines and closes them, which is what pushes the
	// periodic reader's only collection out on an hour-long interval.
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	recorded, err := collector.logs()
	if err != nil {
		t.Fatalf("reading the collector logs = %v", err)
	}
	for _, want := range []string{
		"Name           : orders.place", // the span, by name
		metrics.RequestsTotal,           // the metric, by series
		"service.name: Str(orders)",     // the resource that identifies both
	} {
		if !strings.Contains(recorded, want) {
			t.Errorf("the collector never logged %q; what it logged was:\n%s", want, recorded)
		}
	}
}

func TestWithoutAnInsecureTransportTheHandshakeWithAPlaintextCollectorFails(t *testing.T) {
	ctx := context.Background()

	// The same endpoint, this time over TLS the collector does not speak.
	config := transportConfig(otelboot.Transport{Endpoint: collector.endpoint}, false)

	exporter, err := otlp.TraceExporter(ctx, config)
	if err != nil {
		t.Fatalf("TraceExporter() = %v", err)
	}
	t.Cleanup(func() { _ = exporter.Shutdown(context.Background()) })

	// The exporter retries inside ExportSpans, so the assertion is bounded: the
	// handshake against a plaintext port fails on the first try, and the rest of
	// the budget would only pay for retries of the same failure.
	attempt, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// A real span, not an empty batch: the exporter short-circuits an empty one
	// and would report success without ever opening a connection.
	span := tracetest.SpanStub{
		Name: "orders.place",
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    trace.TraceID{1},
			SpanID:     trace.SpanID{1},
			TraceFlags: trace.FlagsSampled,
		}),
		StartTime: time.Unix(0, 0),
		EndTime:   time.Unix(1, 0),
	}.Snapshot()

	if err := exporter.ExportSpans(attempt, []sdktrace.ReadOnlySpan{span}); err == nil {
		t.Error("ExportSpans() = nil over TLS against a plaintext collector; the negative control proves nothing")
	}
}
