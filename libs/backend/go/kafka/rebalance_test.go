package kafka_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

const otherTopic = "sales.order.cancelled.v1"

type recordingExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error   { return nil }
func (e *recordingExporter) ForceFlush(context.Context) error { return nil }

func (e *recordingExporter) snapshot() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdklog.Record(nil), e.records...)
}

const kafkaScope = "github.com/mateusmacedo/dmpf/libs/backend/go/kafka"

func otlpProvider(level slog.Leveler) (log.LoggerProvider, *recordingExporter) {
	exporter := &recordingExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	return otelboot.Leveled(provider, level), exporter
}

type rebalanceRecord struct {
	body, system, group, destination, scope string
	severity                                log.Severity
}

func rebalanceRecords(records []sdklog.Record) []rebalanceRecord {
	out := make([]rebalanceRecord, 0, len(records))
	for _, record := range records {
		r := rebalanceRecord{body: record.Body().AsString(), scope: record.InstrumentationScope().Name, severity: record.Severity()}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			switch kv.Key {
			case "messaging.system":
				r.system = kv.Value.AsString()
			case "messaging.consumer.group.name":
				r.group = kv.Value.AsString()
			case "messaging.destination.name":
				r.destination = kv.Value.AsString()
			}
			return true
		})
		out = append(out, r)
	}
	return out
}

func TestEveryRebalanceEventLogsOneInfoRecordPerTopicWithThePartitionsInTheBody(t *testing.T) {
	c := newConsumer(newSink())
	provider, exporter := otlpProvider(slog.LevelInfo)
	c.Config.LoggerProvider = provider
	fake := kafka.NewFakeClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	partitions := map[string][]int32{topic: {2, 0}, otherTopic: {1}}

	c.Assign(ctx, fake, partitions)
	c.Revoke(ctx, partitions)
	c.Assign(ctx, fake, partitions)
	c.Lose(ctx, partitions)

	got := map[rebalanceRecord]int{}
	for _, r := range rebalanceRecords(exporter.snapshot()) {
		got[r]++
	}
	group := c.Channel.Group
	want := map[rebalanceRecord]int{
		{body: "partitions assigned [0 2]", system: "kafka", group: group, destination: topic, scope: kafkaScope, severity: log.SeverityInfo}:    2,
		{body: "partitions assigned [1]", system: "kafka", group: group, destination: otherTopic, scope: kafkaScope, severity: log.SeverityInfo}: 2,
		{body: "partitions revoked [0 2]", system: "kafka", group: group, destination: topic, scope: kafkaScope, severity: log.SeverityInfo}:     1,
		{body: "partitions revoked [1]", system: "kafka", group: group, destination: otherTopic, scope: kafkaScope, severity: log.SeverityInfo}:  1,
		{body: "partitions lost [0 2]", system: "kafka", group: group, destination: topic, scope: kafkaScope, severity: log.SeverityInfo}:        1,
		{body: "partitions lost [1]", system: "kafka", group: group, destination: otherTopic, scope: kafkaScope, severity: log.SeverityInfo}:     1,
	}
	if len(got) != len(want) {
		t.Fatalf("records = %v, want one info record per event and topic: %v", got, want)
	}
	for record, n := range want {
		if got[record] != n {
			t.Fatalf("record %+v logged %d times, want %d; all = %v", record, got[record], n, got)
		}
	}
	if c.Workers() != 0 {
		t.Fatalf("workers = %d after the loss, want 0: a lost partition is released like a revoked one", c.Workers())
	}
}

func TestAnEmptyRebalanceLogsNothing(t *testing.T) {
	c := newConsumer(newSink())
	provider, exporter := otlpProvider(slog.LevelInfo)
	c.Config.LoggerProvider = provider
	ctx := context.Background()

	c.Assign(ctx, kafka.NewFakeClient(), map[string][]int32{topic: {}})
	c.Revoke(ctx, map[string][]int32{})
	c.Lose(ctx, map[string][]int32{topic: nil})

	if records := exporter.snapshot(); len(records) != 0 {
		t.Fatalf("exported %d records, want 0: no partition changed hands", len(records))
	}
}
