package sqs_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

var _ interface {
	Publish(context.Context, string, []byte) error
} = (*provider.Publisher)(nil)

func newPublisher(t *testing.T, api provider.SQSAPI) *provider.Publisher {
	t.Helper()
	pub, err := provider.NewPublisher(validConfig(), api)
	if err != nil {
		t.Fatalf("NewPublisher() = %v", err)
	}
	return pub
}

func sent(t *testing.T, api *provider.FakeSQS, i int) *sqs.SendMessageInput {
	t.Helper()
	calls := api.Calls("SendMessage")
	if len(calls) <= i {
		t.Fatalf("SendMessage calls = %d, want more than %d", len(calls), i)
	}
	return calls[i].Input.(*sqs.SendMessageInput)
}

func TestPublishOnFIFODerivesGroupAndDedupAndEncodesOnce(t *testing.T) {
	api := provider.NewFakeSQS()
	pub := newPublisher(t, api)
	raw, env := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "reservations", raw); err != nil {
		t.Fatalf("Publish() = %v", err)
	}
	in := sent(t, api, 0)
	if *in.QueueUrl != fifoURL {
		t.Errorf("QueueUrl = %q, want the catalogued address (TRP-07)", *in.QueueUrl)
	}
	if *in.MessageGroupId != provider.GroupID("k1") || !hex64.MatchString(*in.MessageGroupId) {
		t.Errorf("MessageGroupId = %q, want hex(sha256(k1)) (SQS-05)", *in.MessageGroupId)
	}
	if *in.MessageDeduplicationId != provider.DedupID(env.Source, env.ID, payloadhash.Sum(env.Payload)) {
		t.Errorf("MessageDeduplicationId differs from the derivation of SQS-06")
	}
	decoded, err := provider.DecodeBody(*in.MessageBody)
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatalf("body does not decode once to the envelope: %v (SQS-01)", err)
	}
	if _, ok := in.MessageAttributes[provider.AttrPublishedAt]; !ok || len(in.MessageAttributes) != 1 {
		t.Errorf("attributes = %v, want only %s (TRP-18)", in.MessageAttributes, provider.AttrPublishedAt)
	}
}

func TestPublishOnStandardCarriesNoGroup(t *testing.T) {
	api := provider.NewFakeSQS()
	pub := newPublisher(t, api)
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "notifications", raw); err != nil {
		t.Fatalf("Publish() = %v", err)
	}
	in := sent(t, api, 0)
	if in.MessageGroupId != nil || in.MessageDeduplicationId != nil {
		t.Fatal("a standard queue got FIFO fields (SQS-04)")
	}
}

func TestSameIDDifferentPayloadGetsDifferentDedup(t *testing.T) {
	api := provider.NewFakeSQS()
	pub := newPublisher(t, api)
	raw, env := validRaw(t, "k1")
	env.Payload = []byte{0x0a, 0x03, 'r', '-', '2', 0x10, 0x09}
	other, err := envelope.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.Publish(context.Background(), "reservations", raw); err != nil {
		t.Fatal(err)
	}
	if err := pub.Publish(context.Background(), "reservations", other); err != nil {
		t.Fatal(err)
	}
	if *sent(t, api, 0).MessageDeduplicationId == *sent(t, api, 1).MessageDeduplicationId {
		t.Fatal("the same id with another payload deduplicated: it must reach the inbox as R4 (SQS-06)")
	}
}

func TestPublishRefusesBeforeSending(t *testing.T) {
	api := provider.NewFakeSQS()
	pub := newPublisher(t, api)
	raw, env := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "payments", raw); !errors.Is(err, provider.ErrUnknownChannel) {
		t.Fatalf("unknown = %v, want ErrUnknownChannel", err)
	}
	if err := pub.Publish(context.Background(), "orders-fanout", raw); !errors.Is(err, provider.ErrNotSQSChannel) {
		t.Fatalf("sns channel = %v, want ErrNotSQSChannel", err)
	}
	if err := pub.Publish(context.Background(), "reservations", []byte("garbage")); !errors.Is(err, provider.ErrInvalidEnvelope) {
		t.Fatalf("garbage = %v, want ErrInvalidEnvelope", err)
	}

	env.Payload = []byte(strings.Repeat("x", 200*1024))
	big, err := envelope.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.Publish(context.Background(), "reservations", big); !errors.Is(err, provider.ErrMessageTooLarge) {
		t.Fatalf("262145+ bytes = %v, want ErrMessageTooLarge (SQS-12)", err)
	}
	if len(api.Calls("SendMessage")) != 0 {
		t.Fatal("a refused message reached SendMessage")
	}
}

func TestNewPublisherRefusesAnInvalidConfig(t *testing.T) {
	cfg := validConfig()
	cfg.Catalog = channel.Catalog{}
	if _, err := provider.NewPublisher(cfg, provider.NewFakeSQS()); !errors.Is(err, provider.ErrIncompleteConfig) {
		t.Fatalf("NewPublisher() = %v, want ErrIncompleteConfig", err)
	}
	if _, err := provider.NewPublisher(validConfig(), nil); !errors.Is(err, provider.ErrIncompleteConfig) {
		t.Fatalf("NewPublisher(nil api) = %v, want ErrIncompleteConfig", err)
	}
}
