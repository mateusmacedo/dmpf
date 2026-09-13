package dmpfsqs_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/payloadhash"
	dmpfsqs "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-sqs"
)

// The SQS and SNS rows of the hop matrix of FND-06 §5.2, each as a positive
// and a negative vector.

func TestHopSNSToSQSWithAndWithoutRawDelivery(t *testing.T) {
	raw, env := validRaw(t, "k1")
	body := dmpfsqs.EncodeBody(raw)

	t.Run("positive: raw delivery hands the body through untouched", func(t *testing.T) {
		decoded, err := dmpfsqs.DecodeBody(body)
		if err != nil {
			t.Fatal(err)
		}
		got, err := envelope.Unmarshal(decoded)
		if err != nil || payloadhash.Sum(got.Payload) != payloadhash.Sum(env.Payload) {
			t.Fatal("the raw hop altered the envelope or its hash")
		}
	})

	t.Run("negative: without raw delivery the body is the notification wrapper", func(t *testing.T) {
		wrapped := `{"Type":"Notification","MessageId":"m","TopicArn":"arn:aws:sns:us-east-1:000000000000:t","Message":"` + body + `","Timestamp":"2026-09-06T17:00:00Z"}`
		if _, err := dmpfsqs.DecodeBody(wrapped); !errors.Is(err, dmpfsqs.ErrSNSEnvelopeNotRaw) {
			t.Fatalf("DecodeBody(wrapper) = %v, want ErrSNSEnvelopeNotRaw (SQS-02)", err)
		}
	})
}

func TestHopSQSBase64OnceAndTwice(t *testing.T) {
	raw, env := validRaw(t, "k1")

	t.Run("positive: the publisher encodes once and the consumer decodes once", func(t *testing.T) {
		api := dmpfsqs.NewFakeSQS()
		pub := newPublisher(t, api)
		if err := pub.Publish(context.Background(), "reservations", raw); err != nil {
			t.Fatal(err)
		}
		decoded, err := dmpfsqs.DecodeBody(*sent(t, api, 0).MessageBody)
		if err != nil {
			t.Fatal(err)
		}
		got, err := envelope.Unmarshal(decoded)
		if err != nil || payloadhash.Sum(got.Payload) != payloadhash.Sum(env.Payload) {
			t.Fatal("one encode/decode pair changed the payload (SQS-01, TRP-19)")
		}
	})

	t.Run("negative: a layer that encodes again breaks the envelope", func(t *testing.T) {
		twice := dmpfsqs.EncodeBody([]byte(dmpfsqs.EncodeBody(raw)))
		once, err := dmpfsqs.DecodeBody(twice)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := envelope.Unmarshal(once); err == nil {
			t.Fatal("the doubly encoded body still decoded: the negative vector proves nothing")
		}
	})
}

func TestHopAttributesStayOutsideTheEnvelopeAndTheHash(t *testing.T) {
	raw, env := validRaw(t, "k1")
	api := dmpfsqs.NewFakeSQS()
	pub := newPublisher(t, api)
	if err := pub.Publish(context.Background(), "reservations", raw); err != nil {
		t.Fatal(err)
	}
	in := sent(t, api, 0)

	for key := range in.MessageAttributes {
		if strings.HasPrefix(key, "ce-") || strings.HasPrefix(key, "ce_") {
			t.Errorf("attribute %q moves an envelope attribute out of the body (TRP-18, SQS-03)", key)
		}
	}
	decoded, _ := dmpfsqs.DecodeBody(*in.MessageBody)
	got, _ := envelope.Unmarshal(decoded)
	if payloadhash.Sum(got.Payload) != payloadhash.Sum(env.Payload) {
		t.Fatal("the attributes entered the hash (TRP-20)")
	}
}

func TestHopClaimCheckIsRefusedByTheSizeLimit(t *testing.T) {
	// A payload over the path's limit has no conforming transport (SQS-12b):
	// the publisher refuses instead of replacing the payload by a reference.
	_, env := validRaw(t, "k1")
	env.Payload = []byte(strings.Repeat("x", 300*1024))
	big, err := envelope.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	api := dmpfsqs.NewFakeSQS()
	pub := newPublisher(t, api)
	if err := pub.Publish(context.Background(), "reservations", big); !errors.Is(err, dmpfsqs.ErrMessageTooLarge) {
		t.Fatalf("Publish(big) = %v, want ErrMessageTooLarge (SQS-12, TRP-21)", err)
	}
	if len(api.Calls("SendMessage")) != 0 {
		t.Fatal("something was sent for a payload over the limit")
	}
}
