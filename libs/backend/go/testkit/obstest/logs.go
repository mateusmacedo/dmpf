package obstest

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

type MemoryLogs struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (m *MemoryLogs) Export(_ context.Context, records []sdklog.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, record := range records {
		m.records = append(m.records, record.Clone())
	}
	return nil
}

func (m *MemoryLogs) Shutdown(context.Context) error   { return nil }
func (m *MemoryLogs) ForceFlush(context.Context) error { return nil }

func (m *MemoryLogs) Records() []sdklog.Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]sdklog.Record(nil), m.records...)
}

func (m *MemoryLogs) EventNames() []string {
	records := m.Records()
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.EventName())
	}
	return names
}

func (m *MemoryLogs) Maps() []map[string]any {
	records := m.Records()
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
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
