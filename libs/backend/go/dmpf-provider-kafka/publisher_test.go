package dmpfkafka_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
	dmpfkafka "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-kafka"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

var _ interface {
	Publish(context.Context, string, []byte) error
} = (*dmpfkafka.Publisher)(nil)

// validRaw is a marshalled envelope of the FND-05 profile with an opaque
// payload: the publisher reads the key and never the payload.
func validRaw(t *testing.T, partitionKey string) ([]byte, envelope.Envelope) {
	t.Helper()
	env := envelope.Envelope{
		ID:              "evt-1",
		Source:          "urn:lidercap:sales",
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

func publishConfig() dmpfkafka.Config {
	cfg := validConfig()
	cfg.InsecureForDevelopmentOnly, cfg.TLS = true, nil
	return cfg
}

func TestPublishKeysByPartitionKeyAndPreservesTheBytes(t *testing.T) {
	fake := dmpfkafka.NewFakeClient()
	var observed []dmpfkafka.Record
	pub, err := dmpfkafka.NewPublisherWith(publishConfig(), fake, func(r dmpfkafka.Record) { observed = append(observed, r) })
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
	if len(rec.Headers) != 1 || rec.Headers[0].Key != dmpfkafka.HeaderPublishedAt {
		t.Errorf("Headers = %v, want only %s (TRP-18)", rec.Headers, dmpfkafka.HeaderPublishedAt)
	}
	if len(observed) != 1 || observed[0].Topic != rec.Topic || !bytes.Equal(observed[0].Value, raw) {
		t.Fatalf("observer saw %v", observed)
	}
}

func TestObserverCannotAlterTheRecord(t *testing.T) {
	fake := dmpfkafka.NewFakeClient()
	tamper := func(r dmpfkafka.Record) {
		for i := range r.Value {
			r.Value[i] = 0
		}
		for i := range r.Key {
			r.Key[i] = 'x'
		}
		r.Headers[0].Value[0] = '!'
	}
	pub, err := dmpfkafka.NewPublisherWith(publishConfig(), fake, tamper)
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
	fake := dmpfkafka.NewFakeClient()
	pub, err := dmpfkafka.NewPublisherWith(publishConfig(), fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "payments", raw); !errors.Is(err, dmpfkafka.ErrUnknownChannel) {
		t.Fatalf("unknown destination = %v, want ErrUnknownChannel (ASY-01)", err)
	}
	if err := pub.Publish(context.Background(), "reservations", raw); !errors.Is(err, dmpfkafka.ErrNotKafkaChannel) {
		t.Fatalf("sqs destination = %v, want ErrNotKafkaChannel", err)
	}
	if err := pub.Publish(context.Background(), "orders", []byte("not an envelope")); !errors.Is(err, dmpfkafka.ErrInvalidEnvelope) {
		t.Fatalf("garbage = %v, want ErrInvalidEnvelope", err)
	}
	if len(fake.Produced()) != 0 {
		t.Fatalf("produced %d records, want 0", len(fake.Produced()))
	}
}

func TestPublishReportsTheProducerError(t *testing.T) {
	fake := dmpfkafka.NewFakeClient()
	fake.ProduceErr = errors.New("broker unreachable")
	cfg := publishConfig()
	pub, err := dmpfkafka.NewPublisherWith(cfg, fake, nil)
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
	if _, err := dmpfkafka.NewPublisherWith(cfg, dmpfkafka.NewFakeClient(), nil); !errors.Is(err, dmpfkafka.ErrIncompleteConfig) {
		t.Fatalf("NewPublisherWith() = %v, want ErrIncompleteConfig", err)
	}
}
