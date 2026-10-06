package observe_test

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

const observeScope = "github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"

type memoryExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *memoryExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *memoryExporter) Shutdown(context.Context) error   { return nil }
func (e *memoryExporter) ForceFlush(context.Context) error { return nil }

func (e *memoryExporter) maps() []map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]map[string]any, 0, len(e.records))
	for _, record := range e.records {
		fields := map[string]any{
			"scope": record.InstrumentationScope().Name,
			"level": record.SeverityText(),
			"msg":   record.Body().AsString(),
		}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			fields[string(kv.Key)] = kv.Value.AsInterface()
			return true
		})
		out = append(out, fields)
	}
	return out
}

func (e *memoryExporter) text() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out strings.Builder
	for _, record := range e.records {
		out.WriteString(record.Body().String())
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			out.WriteString(" " + string(kv.Key) + "=" + kv.Value.String())
			return true
		})
		out.WriteString("\n")
	}
	return out.String()
}

func memoryLogs(level slog.Level) (log.LoggerProvider, *memoryExporter) {
	exporter := &memoryExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	return otelboot.Leveled(provider, level), exporter
}
