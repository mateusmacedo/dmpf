package compose_test

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

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
