package grpc_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const grpcScope = "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"

type memoryLogs struct {
	mu      sync.Mutex
	records []map[string]any
}

func newMemoryLogs(level slog.Level) (log.LoggerProvider, *memoryLogs) {
	logs := &memoryLogs{}
	return otelboot.Leveled(sdklog.NewLoggerProvider(sdklog.WithProcessor(logs)), level), logs
}

func (m *memoryLogs) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }
func (m *memoryLogs) Shutdown(context.Context) error                         { return nil }
func (m *memoryLogs) ForceFlush(context.Context) error                       { return nil }

func (m *memoryLogs) OnEmit(ctx context.Context, record *sdklog.Record) error {
	fields := map[string]any{
		"scope": record.InstrumentationScope().Name,
		"level": record.SeverityText(),
		"msg":   record.Body().AsString(),

		"record.trace_id": record.TraceID().String(),
		"record.span_id":  record.SpanID().String(),
	}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		fields[string(kv.Key)] = kv.Value.AsInterface()
		return true
	})
	if execution, ok := ports.ExecutionContextFrom(ctx); ok {
		fields["correlation_id"] = execution.CorrelationID()
		if tenant, scoped := execution.Tenant(); scoped {
			fields["tenant_id"] = string(tenant)
		}
	}
	bag := baggage.FromContext(ctx)
	for _, key := range tracing.ExecutionBaggageKeys {
		if value := bag.Member(key).Value(); value != "" {
			fields["baggage:"+key] = value
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, fields)
	return nil
}

func (m *memoryLogs) snapshot() []map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]map[string]any(nil), m.records...)
}

func listeningAddr(logs *memoryLogs) string {
	for _, record := range logs.snapshot() {
		if record["msg"] == "grpc listening" {
			addr, _ := record["server.address"].(string)
			return addr
		}
	}
	return ""
}

type exportedLogs struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *exportedLogs) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (*exportedLogs) Shutdown(context.Context) error   { return nil }
func (*exportedLogs) ForceFlush(context.Context) error { return nil }

func productionLogs(t *testing.T) (log.LoggerProvider, func(body string) map[string]attribute.Value) {
	t.Helper()
	exporter := &exportedLogs{}
	provider := otelboot.NewLoggerProvider(otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource:   otelboot.Resource{ServiceName: "orders", ServiceVersion: "1.0.0", ServiceInstanceID: "orders-api-1", Role: "api"},
	}, exporter)
	return otelboot.Leveled(provider, slog.LevelInfo), func(body string) map[string]attribute.Value {
		t.Helper()
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() = %v", err)
		}
		exporter.mu.Lock()
		defer exporter.mu.Unlock()
		for _, record := range exporter.records {
			if record.Body().AsString() == body {
				attributes := map[string]attribute.Value{}
				record.WalkAttributes(func(kv attribute.KeyValue) bool {
					attributes[string(kv.Key)] = kv.Value
					return true
				})
				return attributes
			}
		}
		t.Fatalf("exported no %q record", body)
		return nil
	}
}
