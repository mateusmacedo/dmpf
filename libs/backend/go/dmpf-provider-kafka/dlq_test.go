package dmpfkafka_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/twmb/franz-go/pkg/kgo"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfkafka "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/attempt"
)

var _ dmpfports.Containment = (*dmpfkafka.DLQ)(nil)

func TestQuarantinePublishesTheEnvelopeIntactWithDiagnosisHeaders(t *testing.T) {
	fake := dmpfkafka.NewFakeClient()
	dlq, err := dmpfkafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatalf("NewDLQWith() = %v", err)
	}
	raw, _ := validRaw(t, "k1")

	err = dlq.Quarantine(context.Background(), dmpfports.Contained{
		Consumer:  "billing-consumer",
		MessageID: "evt-1",
		Reason:    dmpfports.ReasonAttemptsExhausted,
		Envelope:  raw,
		Error:     "storage: timeout",
		At:        dmpfports.Instant(start.UnixNano()),
	})
	if err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}
	if len(fake.Produced()) != 1 {
		t.Fatalf("produced %d, want 1", len(fake.Produced()))
	}
	rec := fake.Produced()[0]
	if rec.Topic != "sales.order.placed.v1.dlq" {
		t.Errorf("Topic = %q, want the channel's containment (KFK-12)", rec.Topic)
	}
	if !bytes.Equal(rec.Value, raw) {
		t.Error("the dead letter is not the envelope byte for byte (TRP-13, GAR-07)")
	}
	if string(rec.Key) != "k1" {
		t.Errorf("Key = %q, want the partition key so dead letters of one aggregate stay together", rec.Key)
	}
	headers := map[string]string{}
	for _, h := range rec.Headers {
		headers[h.Key] = string(h.Value)
		if strings.HasPrefix(h.Key, "ce-") || strings.HasPrefix(h.Key, "ce_") {
			t.Errorf("header %q moves an envelope attribute beside the envelope (TRP-18)", h.Key)
		}
	}
	want := map[string]string{
		dmpfkafka.HeaderReason:    "attempts-exhausted",
		dmpfkafka.HeaderConsumer:  "billing-consumer",
		dmpfkafka.HeaderMessageID: "evt-1",
		dmpfkafka.HeaderError:     "storage: timeout",
	}
	for k, v := range want {
		if headers[k] != v {
			t.Errorf("header %s = %q, want %q", k, headers[k], v)
		}
	}
	if headers[dmpfkafka.HeaderContainedAt] == "" {
		t.Error("header dmpf-contained-at is missing")
	}
}

func TestQuarantineOfAnUndecodableEnvelopeHasNoKey(t *testing.T) {
	fake := dmpfkafka.NewFakeClient()
	dlq, err := dmpfkafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	garbage := []byte("not an envelope")
	if err := dlq.Quarantine(context.Background(), dmpfports.Contained{Consumer: "c", Reason: dmpfports.ReasonInvalidEnvelope, Envelope: garbage}); err != nil {
		t.Fatal(err)
	}
	if len(fake.Produced()[0].Key) != 0 || !bytes.Equal(fake.Produced()[0].Value, garbage) {
		t.Fatal("an undecodable envelope must be kept as is, without a key")
	}
}

func TestQuarantineRefusesAnIncompleteContainment(t *testing.T) {
	fake := dmpfkafka.NewFakeClient()
	dlq, err := dmpfkafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]dmpfports.Contained{
		"no consumer": {Reason: dmpfports.ReasonCollision, Envelope: []byte("x")},
		"no reason":   {Consumer: "c", Envelope: []byte("x")},
		"no envelope": {Consumer: "c", Reason: dmpfports.ReasonCollision},
	} {
		if err := dlq.Quarantine(context.Background(), c); !errors.Is(err, dmpfkafka.ErrInvalidContainment) {
			t.Errorf("%s: Quarantine() = %v, want ErrInvalidContainment", name, err)
		}
	}
	if len(fake.Produced()) != 0 {
		t.Fatal("an invalid containment reached the broker")
	}
}

func TestNewDLQWithRefusesANonKafkaChannel(t *testing.T) {
	if _, err := dmpfkafka.NewDLQWith(publishConfig(), sqsChannel(), dmpfkafka.NewFakeClient()); !errors.Is(err, dmpfkafka.ErrNotKafkaChannel) {
		t.Fatalf("NewDLQWith(sqs) = %v, want ErrNotKafkaChannel", err)
	}
}

func TestQuarantineCarriesTheAttemptOnlyWhenTheContextHasIt(t *testing.T) {
	fake := dmpfkafka.NewFakeClient()
	dlq, err := dmpfkafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	contained := dmpfports.Contained{Consumer: "c", Reason: dmpfports.ReasonAttemptsExhausted, Envelope: raw}

	if err := dlq.Quarantine(context.Background(), contained); err != nil {
		t.Fatal(err)
	}
	if err := dlq.Quarantine(attempt.WithContext(context.Background(), 3), contained); err != nil {
		t.Fatal(err)
	}
	find := func(rec *kgo.Record) (string, bool) {
		for _, h := range rec.Headers {
			if h.Key == dmpfkafka.HeaderAttempt {
				return string(h.Value), true
			}
		}
		return "", false
	}
	if _, has := find(fake.Produced()[0]); has {
		t.Fatal("a containment without attempt in the context carries dmpf-attempt")
	}
	if v, has := find(fake.Produced()[1]); !has || v != "3" {
		t.Fatalf("dmpf-attempt = %q, %v; want 3 (TRP-52)", v, has)
	}
}

func TestQuarantineBoundsTheValuesThatComeFromOutside(t *testing.T) {
	fake := dmpfkafka.NewFakeClient()
	dlq, err := dmpfkafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	long := strings.Repeat("é", dmpfkafka.HeaderValueLimit)
	contained := dmpfports.Contained{Consumer: "c", Reason: dmpfports.ReasonTerminalFailure, Envelope: raw, MessageID: dmpfports.MessageID(long), Error: long}
	if err := dlq.Quarantine(context.Background(), contained); err != nil {
		t.Fatal(err)
	}
	for _, h := range fake.Produced()[0].Headers {
		if h.Key != dmpfkafka.HeaderMessageID && h.Key != dmpfkafka.HeaderError {
			continue
		}
		if len(h.Value) > dmpfkafka.HeaderValueLimit || !utf8.Valid(h.Value) {
			t.Fatalf("%s = %d bytes (valid utf-8: %v), want at most %d and valid", h.Key, len(h.Value), utf8.Valid(h.Value), dmpfkafka.HeaderValueLimit)
		}
	}
}
