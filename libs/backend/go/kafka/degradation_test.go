package kafka_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

var refusingResource = otelboot.Resource{ServiceName: "reservations", ServiceVersion: "1.0.0", ServiceInstanceID: "reservations-0", Role: "consumer"}

func instrument(collector *obstest.Refusal, cfg *kafka.Config) {
	cfg.Tracer = collector.Runtime.Tracer()
	cfg.Instruments = collector.Runtime.Instruments()
	cfg.LoggerProvider = collector.Runtime.LoggerProvider()
}

func TestAnUnreachableCollectorDoesNotFailThePublication(t *testing.T) {
	collector := obstest.RefusedExports(t, refusingResource)
	cfg := publishConfig()
	instrument(collector, &cfg)
	fake := kafka.NewFakeClient()
	pub, err := kafka.NewPublisherWith(cfg, fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "orders", raw); err != nil {
		t.Fatalf("Publish() = %v with the collector down, want nil: telemetry never fails the publication (NF Degradação)", err)
	}
	collector.RequireLogsAndMetricsRefused(t)
	collector.RequireSpansRefused(t)
	if err := pub.Publish(context.Background(), "orders", raw); err != nil {
		t.Fatalf("Publish() after the refused exports = %v, want nil", err)
	}
	if n := len(fake.Produced()); n != 2 {
		t.Fatalf("produced %d records, want 2", n)
	}
}

func TestAnUnreachableCollectorDoesNotFailTheConsumption(t *testing.T) {
	collector := obstest.RefusedExports(t, refusingResource)
	sink := newSink()
	sink.on(0, func(ctx context.Context, _ int, ack ports.Acknowledger) error {
		_ = ack.Ack(ctx)
		return unavailable
	})
	sink.on(1, alwaysAck)
	c := newConsumer(sink)
	instrument(collector, &c.Config)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)
	flushed := make(chan struct{})

	consume(t, c, fake, func() bool {
		select {
		case <-flushed:
			return len(fake.Commits()) >= 2
		default:
		}
		if len(fake.Commits()) < 1 {
			return false
		}
		collector.RequireLogsAndMetricsRefused(t)
		close(flushed)
		fake.Feed(topic, 0, recordsAt(1)...)
		return false
	})

	if got := commits(fake); len(got) != 2 || got[1] != 1 {
		t.Fatalf("commits = %v, want both records committed with the collector down (NF Degradação)", got)
	}
}
