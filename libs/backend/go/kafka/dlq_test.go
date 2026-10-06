package kafka_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/attempt"
)

var _ ports.Containment = (*kafka.DLQ)(nil)

func TestQuarantinePublishesTheEnvelopeIntactWithDiagnosisHeaders(t *testing.T) {
	fake := kafka.NewFakeClient()
	dlq, err := kafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatalf("NewDLQWith() = %v", err)
	}
	raw, _ := validRaw(t, "k1")

	err = dlq.Quarantine(context.Background(), ports.Contained{
		Consumer:  "billing-consumer",
		MessageID: "evt-1",
		Reason:    ports.ReasonAttemptsExhausted,
		Envelope:  raw,
		Error:     "TransientDependency",
		At:        ports.Instant(start.UnixNano()),
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
		kafka.HeaderReason:    "attempts-exhausted",
		kafka.HeaderConsumer:  "billing-consumer",
		kafka.HeaderMessageID: "evt-1",
		kafka.HeaderError:     "TransientDependency",
	}
	for k, v := range want {
		if headers[k] != v {
			t.Errorf("header %s = %q, want %q", k, headers[k], v)
		}
	}
	if headers[kafka.HeaderContainedAt] == "" {
		t.Error("header dmpf-contained-at is missing")
	}
}

func TestQuarantineOfAnUndecodableEnvelopeHasNoKey(t *testing.T) {
	fake := kafka.NewFakeClient()
	dlq, err := kafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	garbage := []byte("not an envelope")
	if err := dlq.Quarantine(context.Background(), ports.Contained{Consumer: "c", Reason: ports.ReasonInvalidEnvelope, Envelope: garbage}); err != nil {
		t.Fatal(err)
	}
	if len(fake.Produced()[0].Key) != 0 || !bytes.Equal(fake.Produced()[0].Value, garbage) {
		t.Fatal("an undecodable envelope must be kept as is, without a key")
	}
}

func TestQuarantineRefusesAnIncompleteContainment(t *testing.T) {
	fake := kafka.NewFakeClient()
	dlq, err := kafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]ports.Contained{
		"no consumer": {Reason: ports.ReasonCollision, Envelope: []byte("x")},
		"no reason":   {Consumer: "c", Envelope: []byte("x")},
		"no envelope": {Consumer: "c", Reason: ports.ReasonCollision},
	} {
		if err := dlq.Quarantine(context.Background(), c); !errors.Is(err, kafka.ErrInvalidContainment) {
			t.Errorf("%s: Quarantine() = %v, want ErrInvalidContainment", name, err)
		}
	}
	if len(fake.Produced()) != 0 {
		t.Fatal("an invalid containment reached the broker")
	}
}

func TestNewDLQWithRefusesANonKafkaChannel(t *testing.T) {
	if _, err := kafka.NewDLQWith(publishConfig(), sqsChannel(), kafka.NewFakeClient()); !errors.Is(err, kafka.ErrNotKafkaChannel) {
		t.Fatalf("NewDLQWith(sqs) = %v, want ErrNotKafkaChannel", err)
	}
}

func TestQuarantineCarriesTheAttemptOnlyWhenTheContextHasIt(t *testing.T) {
	fake := kafka.NewFakeClient()
	dlq, err := kafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	contained := ports.Contained{Consumer: "c", Reason: ports.ReasonAttemptsExhausted, Envelope: raw}

	if err := dlq.Quarantine(context.Background(), contained); err != nil {
		t.Fatal(err)
	}
	if err := dlq.Quarantine(attempt.WithContext(context.Background(), 3), contained); err != nil {
		t.Fatal(err)
	}
	find := func(rec *kgo.Record) (string, bool) {
		for _, h := range rec.Headers {
			if h.Key == kafka.HeaderAttempt {
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
	fake := kafka.NewFakeClient()
	dlq, err := kafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	long := strings.Repeat("é", kafka.HeaderValueLimit)
	contained := ports.Contained{Consumer: "c", Reason: ports.ReasonTerminalFailure, Envelope: raw, MessageID: ports.MessageID(long), Error: long}
	if err := dlq.Quarantine(context.Background(), contained); err != nil {
		t.Fatal(err)
	}
	for _, h := range fake.Produced()[0].Headers {
		if h.Key != kafka.HeaderMessageID && h.Key != kafka.HeaderError {
			continue
		}
		if len(h.Value) > kafka.HeaderValueLimit || !utf8.Valid(h.Value) {
			t.Fatalf("%s = %d bytes (valid utf-8: %v), want at most %d and valid", h.Key, len(h.Value), utf8.Valid(h.Value), kafka.HeaderValueLimit)
		}
	}
}

func headerValue(rec *kgo.Record, key string) (string, bool) {
	for _, h := range rec.Headers {
		if h.Key == key {
			return string(h.Value), true
		}
	}
	return "", false
}

func TestQuarantineCarriesTheCategoryAndNeverTheFreeTextOfTheError(t *testing.T) {
	fake := kafka.NewFakeClient()
	dlq, err := kafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	for _, text := range []string{"storage: timeout", "app: source outside the trusted boundary", "R1×D4", "card 4111111111111111 declined"} {
		if err := dlq.Quarantine(context.Background(), ports.Contained{Consumer: "c", Reason: ports.ReasonTerminalFailure, Envelope: raw, Error: text}); err != nil {
			t.Fatal(err)
		}
		rec := fake.Produced()[len(fake.Produced())-1]
		if got, _ := headerValue(rec, kafka.HeaderError); got != semconv.ErrorTypeOther.Value.AsString() {
			t.Errorf("Error %q: header %s = %q, want %s, never the text (DAT-03, RF-B1)", text, kafka.HeaderError, got, semconv.ErrorTypeOther.Value.AsString())
		}
	}
}

func TestQuarantineCarriesEveryFND07CategoryAsIs(t *testing.T) {
	fake := kafka.NewFakeClient()
	dlq, err := kafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	for _, category := range []string{"Validation", "DomainRejection", "NotFound", "Conflict", "Forbidden", "Unauthenticated", "TransientDependency", "RateLimited", "DeadlineExceeded", "Cancelled", "Unexpected"} {
		if err := dlq.Quarantine(context.Background(), ports.Contained{Consumer: "c", Reason: ports.ReasonTerminalFailure, Envelope: raw, Error: category}); err != nil {
			t.Fatal(err)
		}
		if got, _ := headerValue(fake.Produced()[len(fake.Produced())-1], kafka.HeaderError); got != category {
			t.Errorf("header %s = %q, want the FND-07 category %q", kafka.HeaderError, got, category)
		}
	}
}

func TestQuarantineWithoutErrorCarriesNoErrorHeader(t *testing.T) {
	fake := kafka.NewFakeClient()
	dlq, err := kafka.NewDLQWith(publishConfig(), ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	if err := dlq.Quarantine(context.Background(), ports.Contained{Consumer: "c", Reason: ports.ReasonCollision, Envelope: raw}); err != nil {
		t.Fatal(err)
	}
	if got, has := headerValue(fake.Produced()[0], kafka.HeaderError); has {
		t.Fatalf("header %s = %q on a containment without error", kafka.HeaderError, got)
	}
}

type containedRecord struct {
	messageID, reason string
	severity          log.Severity
	keys              []string
}

func containedRecords(t *testing.T, exporter *recordingExporter) []containedRecord {
	t.Helper()
	var out []containedRecord
	for _, record := range exporter.snapshot() {
		if record.Body().AsString() != "message contained" {
			continue
		}
		r := containedRecord{severity: record.Severity()}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			r.keys = append(r.keys, string(kv.Key))
			switch kv.Key {
			case "messaging.message.id":
				r.messageID = kv.Value.AsString()
			case "dmpf.containment.reason":
				r.reason = kv.Value.AsString()
			}
			return true
		})
		out = append(out, r)
	}
	return out
}

func TestQuarantineLogsMessageContainedInWarnWithTheMessageIDAndTheReason(t *testing.T) {
	cfg := publishConfig()
	provider, exporter := otlpProvider(slog.LevelInfo)
	cfg.LoggerProvider = provider
	dlq, err := kafka.NewDLQWith(cfg, ordersChannel(), kafka.NewFakeClient())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	if err := dlq.Quarantine(context.Background(), ports.Contained{
		Consumer: "billing-consumer", MessageID: "evt-1", Reason: ports.ReasonAttemptsExhausted, Envelope: raw, Error: "storage: timeout",
	}); err != nil {
		t.Fatal(err)
	}

	records := containedRecords(t, exporter)
	if len(records) != 1 {
		t.Fatalf("message contained records = %+v, want one", records)
	}
	got := records[0]
	if got.severity != log.SeverityWarn {
		t.Errorf("message contained at %v, want warn (RF-A6)", got.severity)
	}
	if got.messageID != "evt-1" || got.reason != "attempts-exhausted" {
		t.Errorf("messaging.message.id = %q, dmpf.containment.reason = %q; want evt-1 and attempts-exhausted", got.messageID, got.reason)
	}
	for _, key := range got.keys {
		if key != "messaging.message.id" && key != "dmpf.containment.reason" {
			t.Errorf("unexpected key %q in message contained", key)
		}
	}
}

func TestQuarantineWithoutMessageIDLogsNoMessageIDKey(t *testing.T) {
	cfg := publishConfig()
	provider, exporter := otlpProvider(slog.LevelInfo)
	cfg.LoggerProvider = provider
	dlq, err := kafka.NewDLQWith(cfg, ordersChannel(), kafka.NewFakeClient())
	if err != nil {
		t.Fatal(err)
	}
	if err := dlq.Quarantine(context.Background(), ports.Contained{Consumer: "c", Reason: ports.ReasonInvalidEnvelope, Envelope: []byte("garbage")}); err != nil {
		t.Fatal(err)
	}
	records := containedRecords(t, exporter)
	if len(records) != 1 || records[0].reason != "invalid-envelope" {
		t.Fatalf("records = %+v, want one message contained with the reason", records)
	}
	for _, key := range records[0].keys {
		if key == "messaging.message.id" {
			t.Fatal("messaging.message.id is present without a message id; a missing key stays absent (CTX-26)")
		}
	}
}

func TestQuarantineThatDoesNotReachTheBrokerLogsNothing(t *testing.T) {
	cfg := publishConfig()
	provider, exporter := otlpProvider(slog.LevelInfo)
	cfg.LoggerProvider = provider
	fake := kafka.NewFakeClient()
	fake.ProduceErr = errors.New("broker down")
	dlq, err := kafka.NewDLQWith(cfg, ordersChannel(), fake)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	if err := dlq.Quarantine(context.Background(), ports.Contained{Consumer: "c", MessageID: "evt-1", Reason: ports.ReasonCollision, Envelope: raw}); err == nil {
		t.Fatal("Quarantine() = nil with the broker down")
	}
	if err := dlq.Quarantine(context.Background(), ports.Contained{Reason: ports.ReasonCollision, Envelope: raw}); !errors.Is(err, kafka.ErrInvalidContainment) {
		t.Fatalf("Quarantine() = %v, want ErrInvalidContainment", err)
	}
	if records := containedRecords(t, exporter); len(records) != 0 {
		t.Fatalf("records = %+v, want none: a message that was not contained is not reported as contained", records)
	}
}
