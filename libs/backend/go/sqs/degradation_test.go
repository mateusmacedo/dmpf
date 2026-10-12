package sqs_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

var refusingResource = otelboot.Resource{ServiceName: "reservations", ServiceVersion: "1.0.0", ServiceInstanceID: "reservations-0", Role: "consumer"}

func instrument(collector *obstest.Refusal, cfg *provider.Config) {
	cfg.Tracer = collector.Runtime.Tracer()
	cfg.Instruments = collector.Runtime.Instruments()
	cfg.LoggerProvider = collector.Runtime.LoggerProvider()
}

func TestAnUnreachableCollectorDoesNotFailThePublication(t *testing.T) {
	collector := obstest.RefusedExports(t, refusingResource)
	cfg := validConfig()
	instrument(collector, &cfg)
	api := provider.NewFakeSQS()
	pub, err := provider.NewPublisher(cfg, api)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "reservations", raw); err != nil {
		t.Fatalf("Publish() = %v with the collector down, want nil: telemetry never fails the publication (NF Degradação)", err)
	}
	collector.RequireLogsAndMetricsRefused(t)
	collector.RequireSpansRefused(t)
	if err := pub.Publish(context.Background(), "reservations", raw); err != nil {
		t.Fatalf("Publish() after the refused exports = %v, want nil", err)
	}
	if n := len(api.Calls("SendMessage")); n != 2 {
		t.Fatalf("sent %d messages, want 2", n)
	}
}

func TestAnUnreachableCollectorDoesNotFailTheConsumption(t *testing.T) {
	collector := obstest.RefusedExports(t, refusingResource)
	api := provider.NewFakeSQS()
	sink := newSink(func(ctx context.Context, _ handled, ack ports.Acknowledger) error {
		_ = ack.Ack(ctx)
		return throttled
	})
	consumer := newConsumer(clock.NewFake(start), sink)
	instrument(collector, &consumer.Config)
	stop := run(t, consumer, api)
	defer stop()
	raw, _ := validRaw(t, "k1")

	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-1", provider.EncodeBody(raw), "1")}})
	awaitCalls(t, api, "DeleteMessage", 1)
	collector.RequireLogsAndMetricsRefused(t)
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-2", provider.EncodeBody(raw), "1")}})

	if deletes := awaitCalls(t, api, "DeleteMessage", 2); len(deletes) != 2 {
		t.Fatalf("deleted %d messages, want both consumed with the collector down (NF Degradação)", len(deletes))
	}
}
