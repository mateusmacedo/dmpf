package kafka_test

import (
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
)

func TestTheProducerWaitsForEveryInSyncReplicaAndWritesIdempotently(t *testing.T) {
	acks, idempotenceDisabled, err := kafka.ProducerOptionsFor(kafka.Config{Brokers: []string{"localhost:1"}})
	if err != nil {
		t.Fatalf("ProducerOptionsFor() = %v, want nil", err)
	}
	if acks != kgo.AllISRAcks() {
		t.Errorf("RequiredAcks = %v, want AllISRAcks: a leader-only ack loses what the relay already marked published", acks)
	}
	if idempotenceDisabled {
		t.Error("DisableIdempotentWrite is set: the producer's own retries would duplicate records in the topic")
	}
}
