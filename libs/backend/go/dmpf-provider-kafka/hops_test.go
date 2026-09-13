package dmpfkafka_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/payloadhash"
	dmpfkafka "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-kafka"
)

// The Kafka rows of the hop matrix of FND-06 §5.2, each as a positive and a
// negative vector: the positive is what this provider does, the negative is the
// path the matrix marks non-conforming, reproduced as a fixture.

func hashOf(t *testing.T, raw []byte) string {
	t.Helper()
	env, err := envelope.Unmarshal(raw)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return payloadhash.Sum(env.Payload)
}

func publishThrough(t *testing.T, observer dmpfkafka.Observer, raw []byte) *kgo.Record {
	t.Helper()
	fake := dmpfkafka.NewFakeClient()
	pub, err := dmpfkafka.NewPublisherWith(publishConfig(), fake, observer)
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.Publish(context.Background(), "orders", raw); err != nil {
		t.Fatalf("Publish() = %v", err)
	}
	return fake.Produced()[0]
}

func TestHopDirectPublicationPreservesThePayloadHash(t *testing.T) {
	raw, _ := validRaw(t, "k1")
	produced := publishThrough(t, nil, raw)

	if !bytes.Equal(produced.Value, raw) {
		t.Fatal("record.value differs from the published bytes (§5.2 row 1)")
	}
	if hashOf(t, produced.Value) != hashOf(t, raw) {
		t.Fatal("payload_hash after the hop differs (ENV-18)")
	}
}

func TestHopInterceptorMayReadButTheReserializingOneIsNotConforming(t *testing.T) {
	raw, _ := validRaw(t, "k1")

	t.Run("positive: the observer reads and the bytes stay", func(t *testing.T) {
		var seen []byte
		produced := publishThrough(t, func(r dmpfkafka.Record) { seen = r.Value }, raw)
		if !bytes.Equal(seen, raw) || !bytes.Equal(produced.Value, raw) {
			t.Fatal("the observer saw or produced other bytes (TRP-17)")
		}
	})

	t.Run("negative: a reserializing interceptor changes the hash", func(t *testing.T) {
		// A producer interceptor that decodes the message and re-encodes it
		// writes the payload's fields in another order: the same message,
		// other bytes — the row the matrix marks non-conforming.
		env, err := envelope.Unmarshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		reordered := append(append([]byte(nil), env.Payload[5:]...), env.Payload[:5]...)
		env.Payload = reordered
		reserialized, err := envelope.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		if hashOf(t, reserialized) == hashOf(t, raw) {
			t.Fatal("the fixture did not diverge: the negative vector proves nothing")
		}
		produced := publishThrough(t, nil, raw)
		if hashOf(t, produced.Value) == hashOf(t, reserialized) {
			t.Fatal("the provider produced the reserialized bytes")
		}
	})
}

func TestHopHeadersCarryOperationalMetadataAndNeverAnEnvelopeAttribute(t *testing.T) {
	raw, env := validRaw(t, "k1")

	t.Run("positive: the operational header leaves the envelope self-contained", func(t *testing.T) {
		produced := publishThrough(t, nil, raw)
		if len(produced.Headers) == 0 || produced.Headers[0].Key != dmpfkafka.HeaderPublishedAt {
			t.Fatalf("headers = %v, want the operational header (TRP-18)", produced.Headers)
		}
		decoded, err := envelope.Unmarshal(produced.Value)
		if err != nil {
			t.Fatalf("the envelope no longer decodes on its own: %v", err)
		}
		if decoded.PartitionKey != env.PartitionKey || payloadhash.Sum(decoded.Payload) != payloadhash.Sum(env.Payload) {
			t.Fatal("the header hop altered the envelope")
		}
	})

	t.Run("negative: an attribute moved to a header breaks the envelope", func(t *testing.T) {
		moved := env
		moved.PartitionKey = ""
		stripped, err := envelope.Marshal(moved)
		if err == nil {
			_, err = envelope.Unmarshal(stripped)
		}
		if err == nil {
			t.Fatal("an envelope without its partition key still validates: the negative vector proves nothing (ENV-08)")
		}
	})
}
