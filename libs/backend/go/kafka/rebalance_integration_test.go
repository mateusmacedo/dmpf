//go:build integration

package kafka_test

import (
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/log"
)

func TestIntegrationJoiningAndLeavingTheGroupLogOneRecordPerTopic(t *testing.T) {
	seeds := brokers(t)
	ch := integrationChannel(uniqueSuffix())
	createTopics(t, seeds, int32(ch.Partitions), ch.Address, ch.Containment)
	cfg := integrationConfig(seeds, ch)
	provider, exporter := otlpProvider(slog.LevelInfo)
	cfg.LoggerProvider = provider
	assigned := rebalanceRecord{body: "partitions assigned [0 1 2]", system: "kafka", group: ch.Group, destination: ch.Address, scope: kafkaScope, severity: log.SeverityInfo}
	revoked := assigned
	revoked.body = "partitions revoked [0 1 2]"
	count := func(want rebalanceRecord) int {
		n := 0
		for _, r := range rebalanceRecords(exporter.snapshot()) {
			if r == want {
				n++
			}
		}
		return n
	}

	sink := &recordingSink{perKey: map[string][]int{}, hashes: map[string]string{}}
	stop, done := runConsumer(t, cfg, ch, sink)
	waitFor(t, "the assignment logged", 60*time.Second, func() bool { return count(assigned) > 0 })
	stop()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("the consumer did not leave the group")
	}

	if got := count(assigned); got != 1 {
		t.Fatalf("assignment logged %d times, want 1; records = %v", got, rebalanceRecords(exporter.snapshot()))
	}
	if got := count(revoked); got != 1 {
		t.Fatalf("revocation on leaving logged %d times, want 1; records = %v", got, rebalanceRecords(exporter.snapshot()))
	}
	if got := len(rebalanceRecords(exporter.snapshot())); got != 2 {
		t.Fatalf("exported %d records, want only the assignment and the revocation: %v", got, rebalanceRecords(exporter.snapshot()))
	}
}
