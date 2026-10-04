package kafka_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

var _ interface {
	Publish(context.Context, string, []byte) error
} = (*kafka.Publisher)(nil)

// validRaw is a marshalled envelope of the FND-05 profile with an opaque
// payload: the publisher reads the key and never the payload.
func validRaw(t *testing.T, partitionKey string) ([]byte, envelope.Envelope) {
	t.Helper()
	env := envelope.Envelope{
		ID:              "evt-1",
		Source:          "urn:dmpf:sales",
		SpecVersion:     envelope.SpecVersion,
		Type:            "sales.order.placed.v1",
		Subject:         "order/" + partitionKey,
		Time:            timestamppb.New(start),
		DataSchema:      "type.googleapis.com/sales.order.v1.OrderPlaced",
		DataContentType: envelope.ContentType,
		CorrelationID:   "corr-1",
		CausationID:     "evt-0",
		PartitionKey:    partitionKey,
		TraceParent:     "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		Payload:         []byte{0x0a, 0x03, 'o', '-', '1', 0x10, 0x02},
	}
	raw, err := envelope.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return raw, env
}

func publishConfig() kafka.Config {
	cfg := validConfig()
	cfg.InsecureForDevelopmentOnly, cfg.TLS, cfg.SASL = true, nil, nil
	return cfg
}

func TestPublishKeysByPartitionKeyAndPreservesTheBytes(t *testing.T) {
	fake := kafka.NewFakeClient()
	var observed []kafka.Record
	pub, err := kafka.NewPublisherWith(publishConfig(), fake, func(r kafka.Record) { observed = append(observed, r) })
	if err != nil {
		t.Fatalf("NewPublisherWith() = %v", err)
	}
	raw, env := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "orders", raw); err != nil {
		t.Fatalf("Publish() = %v, want nil", err)
	}
	if len(fake.Produced()) != 1 {
		t.Fatalf("produced %d records, want 1", len(fake.Produced()))
	}
	rec := fake.Produced()[0]
	if rec.Topic != "sales.order.placed.v1" {
		t.Errorf("Topic = %q, want the catalogued address (TRP-07)", rec.Topic)
	}
	if string(rec.Key) != "k1" {
		t.Errorf("Key = %q, want the envelope's partition key (KFK-05)", rec.Key)
	}
	if !bytes.Equal(rec.Value, raw) {
		t.Error("Value differs from the message byte for byte (TRP-13)")
	}
	decoded, err := envelope.Unmarshal(rec.Value)
	if err != nil {
		t.Fatal(err)
	}
	if payloadhash.Sum(decoded.Payload) != payloadhash.Sum(env.Payload) {
		t.Error("payload hash after the hop differs from the published one (ENV-18)")
	}
	if len(rec.Headers) != 2 || rec.Headers[0].Key != kafka.HeaderPublishedAt ||
		rec.Headers[1].Key != "content-type" || string(rec.Headers[1].Value) != "application/cloudevents+protobuf" {
		t.Errorf("Headers = %v, want only %s (TRP-18) and the content-type of the CloudEvents binding (RF-B7)", rec.Headers, kafka.HeaderPublishedAt)
	}
	if len(observed) != 1 || observed[0].Topic != rec.Topic || !bytes.Equal(observed[0].Value, raw) {
		t.Fatalf("observer saw %v", observed)
	}
}

func TestObserverCannotAlterTheRecord(t *testing.T) {
	fake := kafka.NewFakeClient()
	tamper := func(r kafka.Record) {
		for i := range r.Value {
			r.Value[i] = 0
		}
		for i := range r.Key {
			r.Key[i] = 'x'
		}
		r.Headers[0].Value[0] = '!'
	}
	pub, err := kafka.NewPublisherWith(publishConfig(), fake, tamper)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "orders", raw); err != nil {
		t.Fatal(err)
	}
	rec := fake.Produced()[0]
	if !bytes.Equal(rec.Value, raw) || string(rec.Key) != "k1" || rec.Headers[0].Value[0] == '!' {
		t.Fatal("the observer altered what was produced (TRP-17)")
	}
}

func TestPublishRefusesBeforeProducing(t *testing.T) {
	fake := kafka.NewFakeClient()
	pub, err := kafka.NewPublisherWith(publishConfig(), fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "payments", raw); !errors.Is(err, kafka.ErrUnknownChannel) {
		t.Fatalf("unknown destination = %v, want ErrUnknownChannel (ASY-01)", err)
	}
	if err := pub.Publish(context.Background(), "reservations", raw); !errors.Is(err, kafka.ErrNotKafkaChannel) {
		t.Fatalf("sqs destination = %v, want ErrNotKafkaChannel", err)
	}
	if err := pub.Publish(context.Background(), "orders", []byte("not an envelope")); !errors.Is(err, kafka.ErrInvalidEnvelope) {
		t.Fatalf("garbage = %v, want ErrInvalidEnvelope", err)
	}
	if len(fake.Produced()) != 0 {
		t.Fatalf("produced %d records, want 0", len(fake.Produced()))
	}
}

func TestPublishReportsTheProducerError(t *testing.T) {
	fake := kafka.NewFakeClient()
	fake.ProduceErr = errors.New("broker unreachable")
	cfg := publishConfig()
	pub, err := kafka.NewPublisherWith(cfg, fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := pub.Publish(ctx, "orders", raw); err == nil {
		t.Fatal("Publish() = nil, want the producer's error")
	}
}

func TestNewPublisherWithRefusesAnInvalidConfig(t *testing.T) {
	cfg := publishConfig()
	cfg.Catalog = channel.Catalog{}
	if _, err := kafka.NewPublisherWith(cfg, kafka.NewFakeClient(), nil); !errors.Is(err, kafka.ErrIncompleteConfig) {
		t.Fatalf("NewPublisherWith() = %v, want ErrIncompleteConfig", err)
	}
}

func tracedPublisher(t *testing.T, fake *kafka.FakeClient) (*kafka.Publisher, *tracetest.SpanRecorder, trace.Tracer) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("kafka-test")
	cfg := publishConfig()
	cfg.Tracer = tracer
	pub, err := kafka.NewPublisherWith(cfg, fake, nil)
	if err != nil {
		t.Fatalf("NewPublisherWith() = %v", err)
	}
	return pub, recorder, tracer
}

func attributeOf(span sdktrace.ReadOnlySpan, key attribute.Key) (attribute.Value, bool) {
	for _, kv := range span.Attributes() {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func assertOnlySpan(t *testing.T, recorder *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := recorder.Ended()
	names := make([]string, len(spans))
	for i, span := range spans {
		names[i] = span.Name()
		if strings.HasPrefix(span.Name(), "dmpf.kafka.") {
			t.Errorf("span %q: the publisher names no span of its own (RF-B7)", span.Name())
		}
	}
	if len(spans) != 1 || spans[0].Name() != name {
		t.Fatalf("ended spans = %q, want only %q", names, name)
	}
	return spans[0]
}

func assertPartitionAndOffset(t *testing.T, span sdktrace.ReadOnlySpan, partition string, offset int64) {
	t.Helper()
	if got, _ := attributeOf(span, semconv.MessagingDestinationPartitionIDKey); got.AsString() != partition {
		t.Errorf("messaging.destination.partition.id = %q, want %q (RF-B7)", got.AsString(), partition)
	}
	if got, ok := attributeOf(span, semconv.MessagingKafkaOffsetKey); !ok || got.AsInt64() != offset {
		t.Errorf("messaging.kafka.offset = %v (present %v), want %d (RF-B7)", got.AsInt64(), ok, offset)
	}
}

func TestPublishWritesPartitionAndOffsetOnTheOwnedSend(t *testing.T) {
	fake := kafka.NewFakeClient()
	fake.ProducePartition, fake.ProduceOffset = 3, 41
	pub, recorder, tracer := tracedPublisher(t, fake)
	raw, _ := validRaw(t, "k1")
	ctx, send := tracer.Start(context.Background(), "send relay-owned", trace.WithSpanKind(trace.SpanKindClient))

	err := pub.Publish(tracing.WithOwnedSpan(ctx, send), "orders", raw)
	send.End()
	if err != nil {
		t.Fatalf("Publish() = %v", err)
	}

	span := assertOnlySpan(t, recorder, "send relay-owned")
	assertPartitionAndOffset(t, span, "3", 41)
}

func TestPublishWritesPartitionAndOffsetOnTheResilienceSpanOutsideTheRelay(t *testing.T) {
	fake := kafka.NewFakeClient()
	fake.ProducePartition, fake.ProduceOffset = 7, 1200
	pub, recorder, _ := tracedPublisher(t, fake)
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "orders", raw); err != nil {
		t.Fatalf("Publish() = %v", err)
	}

	span := assertOnlySpan(t, recorder, "dmpf.resilience kafka")
	assertPartitionAndOffset(t, span, "7", 1200)
}

func TestPublishLeavesNoPartitionOrOffsetWhenTheProduceFails(t *testing.T) {
	fake := kafka.NewFakeClient()
	fake.ProduceErr = errors.New("broker unreachable")
	pub, recorder, _ := tracedPublisher(t, fake)
	raw, _ := validRaw(t, "k1")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := pub.Publish(ctx, "orders", raw); err == nil {
		t.Fatal("Publish() = nil, want the producer's error")
	}

	span := assertOnlySpan(t, recorder, "dmpf.resilience kafka")
	for _, key := range []attribute.Key{semconv.MessagingDestinationPartitionIDKey, semconv.MessagingKafkaOffsetKey} {
		if got, ok := attributeOf(span, key); ok {
			t.Errorf("%s = %s on a failed produce, want absent", key, got.String())
		}
	}
}

func TestPublishCarriesTheCloudEventsContentTypeAndNoTraceHeader(t *testing.T) {
	fake := kafka.NewFakeClient()
	pub, err := kafka.NewPublisherWith(publishConfig(), fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "orders", raw); err != nil {
		t.Fatal(err)
	}
	var contentTypes []string
	for _, h := range fake.Produced()[0].Headers {
		switch h.Key {
		case "content-type":
			contentTypes = append(contentTypes, string(h.Value))
		case "traceparent", "tracestate", "baggage":
			t.Errorf("header %s: no W3C header per hop (RF-B7)", h.Key)
		}
	}
	if len(contentTypes) != 1 || contentTypes[0] != "application/cloudevents+protobuf" {
		t.Fatalf("content-type = %q, want application/cloudevents+protobuf once (RF-B7)", contentTypes)
	}
}
