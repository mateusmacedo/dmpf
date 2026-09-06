package dmpfsqs_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
	dmpfsqs "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-sqs"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

var _ interface {
	Publish(context.Context, string, []byte) error
} = (*dmpfsqs.Publisher)(nil)

func newPublisher(t *testing.T, api dmpfsqs.SQSAPI) *dmpfsqs.Publisher {
	t.Helper()
	pub, err := dmpfsqs.NewPublisher(validConfig(), api)
	if err != nil {
		t.Fatalf("NewPublisher() = %v", err)
	}
	return pub
}

func sent(t *testing.T, api *dmpfsqs.FakeSQS, i int) *sqs.SendMessageInput {
	t.Helper()
	calls := api.Calls("SendMessage")
	if len(calls) <= i {
		t.Fatalf("SendMessage calls = %d, want more than %d", len(calls), i)
	}
	return calls[i].Input.(*sqs.SendMessageInput)
}

func TestPublishOnFIFODerivesGroupAndDedupAndEncodesOnce(t *testing.T) {
	api := dmpfsqs.NewFakeSQS()
	pub := newPublisher(t, api)
	raw, env := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "reservations", raw); err != nil {
		t.Fatalf("Publish() = %v", err)
	}
	in := sent(t, api, 0)
	if *in.QueueUrl != fifoURL {
		t.Errorf("QueueUrl = %q, want the catalogued address (TRP-07)", *in.QueueUrl)
	}
	if *in.MessageGroupId != dmpfsqs.GroupID("k1") || !hex64.MatchString(*in.MessageGroupId) {
		t.Errorf("MessageGroupId = %q, want hex(sha256(k1)) (SQS-05)", *in.MessageGroupId)
	}
	if *in.MessageDeduplicationId != dmpfsqs.DedupID(env.Source, env.ID, payloadhash.Sum(env.Payload)) {
		t.Errorf("MessageDeduplicationId differs from the derivation of SQS-06")
	}
	decoded, err := dmpfsqs.DecodeBody(*in.MessageBody)
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatalf("body does not decode once to the envelope: %v (SQS-01)", err)
	}
	if _, ok := in.MessageAttributes[dmpfsqs.AttrPublishedAt]; !ok || len(in.MessageAttributes) != 1 {
		t.Errorf("attributes = %v, want only %s (TRP-18)", in.MessageAttributes, dmpfsqs.AttrPublishedAt)
	}
}

func TestPublishOnStandardCarriesNoGroup(t *testing.T) {
	api := dmpfsqs.NewFakeSQS()
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
	api := dmpfsqs.NewFakeSQS()
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
	api := dmpfsqs.NewFakeSQS()
	pub := newPublisher(t, api)
	raw, env := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "payments", raw); !errors.Is(err, dmpfsqs.ErrUnknownChannel) {
		t.Fatalf("unknown = %v, want ErrUnknownChannel", err)
	}
	if err := pub.Publish(context.Background(), "orders-fanout", raw); !errors.Is(err, dmpfsqs.ErrNotSQSChannel) {
		t.Fatalf("sns channel = %v, want ErrNotSQSChannel", err)
	}
	if err := pub.Publish(context.Background(), "reservations", []byte("garbage")); !errors.Is(err, dmpfsqs.ErrInvalidEnvelope) {
		t.Fatalf("garbage = %v, want ErrInvalidEnvelope", err)
	}

	env.Payload = []byte(strings.Repeat("x", 200*1024))
	big, err := envelope.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.Publish(context.Background(), "reservations", big); !errors.Is(err, dmpfsqs.ErrMessageTooLarge) {
		t.Fatalf("262145+ bytes = %v, want ErrMessageTooLarge (SQS-12)", err)
	}
	if len(api.Calls("SendMessage")) != 0 {
		t.Fatal("a refused message reached SendMessage")
	}
}

func TestNewPublisherRefusesAnInvalidConfig(t *testing.T) {
	cfg := validConfig()
	cfg.Catalog = channel.Catalog{}
	if _, err := dmpfsqs.NewPublisher(cfg, dmpfsqs.NewFakeSQS()); !errors.Is(err, dmpfsqs.ErrIncompleteConfig) {
		t.Fatalf("NewPublisher() = %v, want ErrIncompleteConfig", err)
	}
	if _, err := dmpfsqs.NewPublisher(validConfig(), nil); !errors.Is(err, dmpfsqs.ErrIncompleteConfig) {
		t.Fatalf("NewPublisher(nil api) = %v, want ErrIncompleteConfig", err)
	}
}
