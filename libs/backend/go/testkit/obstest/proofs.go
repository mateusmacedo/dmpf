package obstest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

// NotFoundOnTheUseCase proves that find, run over a pool nobody answers and a
// tenant-scoped call, fails NotFound and leaves NotFound as the error.type of
// the use case span and of its RED, the category the edge answers.
func NotFoundOnTheUseCase(t *testing.T, service, operation string, find func(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool) error) {
	t.Helper()
	ctx := context.Background()
	spans := tracetest.NewInMemoryExporter()
	reader := sdkmetric.NewManualReader()
	runtime, err := otelboot.Start(ctx, otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: service, ServiceVersion: "dev", ServiceInstanceID: service + "-1"},
		TraceExporter: spans,
		MetricReader:  reader,
	})
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	pool, err := pgxpool.New(ctx, "postgres://"+service+"@127.0.0.1:1/"+service)
	if err != nil {
		t.Fatalf("pgxpool.New() = %v, want nil", err)
	}
	defer pool.Close()

	if err := find(execctx.WithExecution(t, ctx, execctx.WithoutSubject()), runtime, pool); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("%s = %v, want %v", operation, err, ports.ErrNotFound)
	}
	if err := runtime.ForceFlush(ctx); err != nil {
		t.Fatalf("ForceFlush() = %v, want nil", err)
	}

	want := string(application.NotFound)
	if got := SpanErrorType(t, spans, "dmpf.usecase."+operation); got != want {
		t.Errorf("span error.type = %q, want %q: the edge answers NotFound for the same error", got, want)
	}
	if got := REDErrorType(t, reader, operation); got != want {
		t.Errorf("RED error.type = %q, want %q", got, want)
	}
}

// RelayWiring is the relay role of an app draining a record of Channel into
// the physical Topic.
type RelayWiring struct {
	Service string
	Scope   string
	Topic   string
	Channel string
	Run     func(ctx context.Context, rt *otelboot.Runtime) error
}

// Prove runs the role until it sends, then holds the send span and the
// "relay draining" record to the physical topic (RF-B7) and to the scope of the
// app package that emits it (RF-A1).
func (w RelayWiring) Prove(t *testing.T) {
	t.Helper()
	spans := tracetest.NewInMemoryExporter()
	logs := &MemoryLogs{}
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: w.Service, ServiceVersion: "dev", ServiceInstanceID: w.Service + "-relay-1", Role: "relay"},
		TraceExporter: spans,
		Sampling:      tracing.UniformRates(1),
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("otelboot.Start() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- w.Run(ctx, runtime) }()
	send := AwaitSpan(t, runtime, spans, "send", 15*time.Second)
	cancel()
	if err := <-stopped; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("RunWith(relay) = %v, want nil or the cancellation", err)
	}
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	if scopes := ScopesOf(logs, "relay draining"); len(scopes) != 1 || scopes[0] != w.Scope {
		t.Fatalf("\"relay draining\" scopes = %v, want the import path of the app package that emits it (RF-A1)", scopes)
	}
	draining, _ := RecordAttributes(logs, "relay draining")
	RequireTheVocabulary(t, "relay draining", draining, map[string]string{
		string(semconv.MessagingSystemKey):          semconv.MessagingSystemKafka.Value.AsString(),
		string(semconv.MessagingDestinationNameKey): w.Topic,
		"dmpf.channel.name":                         w.Channel,
	})
	if send.Name != "send "+w.Topic {
		t.Fatalf("send span = %q, want %q (RF-B7)", send.Name, "send "+w.Topic)
	}
	if got := DestinationOf(send); got != w.Topic {
		t.Fatalf("messaging.destination.name = %q, want the physical topic %q, not the channel %q (RF-B7)", got, w.Topic, w.Channel)
	}
}

func DestinationOf(span tracetest.SpanStub) string {
	for _, kv := range span.Attributes {
		if kv.Key == semconv.MessagingDestinationNameKey {
			return kv.Value.AsString()
		}
	}
	return ""
}

func RecordAttributes(logs *MemoryLogs, body string) (map[string]string, bool) {
	for _, record := range logs.Records() {
		if record.Body().AsString() != body {
			continue
		}
		attributes := map[string]string{}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			attributes[string(kv.Key)] = kv.Value.String()
			return true
		})
		return attributes, true
	}
	return nil, false
}

// RequireTheVocabulary holds a start record to the attributes it must carry
// past the processor (RF-A3) and to none of the keys of an execution (RF-A4).
func RequireTheVocabulary(t testing.TB, body string, attributes map[string]string, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if attributes[key] != value {
			t.Errorf("%q %s = %q, want %q surviving the processor (RF-A3): %v", body, key, attributes[key], value, attributes)
		}
	}
	for _, key := range []string{tracing.KeyCorrelationID, tracing.KeyRequestID, tracing.KeyTenantID} {
		if value, present := attributes[key]; present {
			t.Errorf("%q carries %s = %q, want no key of an execution on a start record (RF-A4)", body, key, value)
		}
	}
}

func ScopesOf(logs *MemoryLogs, body string) []string {
	var scopes []string
	for _, record := range logs.Records() {
		if record.Body().AsString() == body {
			scopes = append(scopes, record.InstrumentationScope().Name)
		}
	}
	return scopes
}
