package dmpfsqs_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfsqs "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-sqs"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/attempt"
)

var _ dmpfports.Containment = (*dmpfsqs.DLQ)(nil)

func TestQuarantineSendsTheEncodedEnvelopeWithDiagnosisAttributes(t *testing.T) {
	api := dmpfsqs.NewFakeSQS()
	dlq, err := dmpfsqs.NewDLQ(validConfig(), api, fifoChannel())
	if err != nil {
		t.Fatalf("NewDLQ() = %v", err)
	}
	raw, _ := validRaw(t, "k1")

	err = dlq.Quarantine(context.Background(), dmpfports.Contained{
		Consumer: "stock-consumer", MessageID: "evt-1", Reason: dmpfports.ReasonTerminalFailure,
		Envelope: raw, Error: "storage: constraint", At: dmpfports.Instant(start.UnixNano()),
	})
	if err != nil {
		t.Fatalf("Quarantine() = %v", err)
	}
	in := sent(t, api, 0)
	if *in.QueueUrl != fifoDLQ {
		t.Errorf("QueueUrl = %q, want the channel's containment (SQS-11)", *in.QueueUrl)
	}
	if *in.MessageBody != dmpfsqs.EncodeBody(raw) {
		t.Error("the dead letter is not the envelope encoded once (GAR-07, TRP-19)")
	}
	if in.MessageGroupId == nil || *in.MessageGroupId != dmpfsqs.GroupID("k1") {
		t.Error("a FIFO containment must derive the group from the envelope")
	}
	for _, key := range []string{dmpfsqs.AttrReason, dmpfsqs.AttrConsumer, dmpfsqs.AttrMessageID, dmpfsqs.AttrContainedAt, dmpfsqs.AttrError} {
		if _, ok := in.MessageAttributes[key]; !ok {
			t.Errorf("attribute %s is missing", key)
		}
	}
	if *in.MessageAttributes[dmpfsqs.AttrReason].StringValue != "terminal-failure" {
		t.Errorf("dmpf-reason = %q", *in.MessageAttributes[dmpfsqs.AttrReason].StringValue)
	}
	if len(in.MessageAttributes) > dmpfsqs.MaxAttributes {
		t.Errorf("%d attributes exceed the ten of SQS-03b", len(in.MessageAttributes))
	}
}

func TestQuarantineOfAnUndecodableEnvelopeStillGoesToAFIFOQueue(t *testing.T) {
	api := dmpfsqs.NewFakeSQS()
	dlq, err := dmpfsqs.NewDLQ(validConfig(), api, fifoChannel())
	if err != nil {
		t.Fatal(err)
	}
	if err := dlq.Quarantine(context.Background(), dmpfports.Contained{Consumer: "c", Reason: dmpfports.ReasonInvalidEnvelope, Envelope: []byte("garbage")}); err != nil {
		t.Fatal(err)
	}
	in := sent(t, api, 0)
	if in.MessageGroupId == nil || in.MessageDeduplicationId == nil {
		t.Fatal("a FIFO containment without group or dedup would be refused by the queue")
	}
	decoded, err := dmpfsqs.DecodeBody(*in.MessageBody)
	if err != nil || string(decoded) != "garbage" {
		t.Fatal("the undecodable envelope was not kept as is")
	}
}

func TestQuarantineRefusesAnIncompleteContainment(t *testing.T) {
	api := dmpfsqs.NewFakeSQS()
	dlq, err := dmpfsqs.NewDLQ(validConfig(), api, standardChannel())
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]dmpfports.Contained{
		"no consumer": {Reason: dmpfports.ReasonCollision, Envelope: []byte("x")},
		"no reason":   {Consumer: "c", Envelope: []byte("x")},
		"no envelope": {Consumer: "c", Reason: dmpfports.ReasonCollision},
	} {
		if err := dlq.Quarantine(context.Background(), c); !errors.Is(err, dmpfsqs.ErrInvalidContainment) {
			t.Errorf("%s: Quarantine() = %v, want ErrInvalidContainment", name, err)
		}
	}
	if len(api.Calls("SendMessage")) != 0 {
		t.Fatal("an invalid containment reached the queue")
	}
	var _ *sqs.SendMessageInput
}

func TestNewDLQRefusesANonSQSChannel(t *testing.T) {
	if _, err := dmpfsqs.NewDLQ(validConfig(), dmpfsqs.NewFakeSQS(), kafkaChannel()); !errors.Is(err, dmpfsqs.ErrNotSQSChannel) {
		t.Fatalf("NewDLQ(kafka) = %v, want ErrNotSQSChannel", err)
	}
}

func TestQuarantineCarriesTheAttemptOnlyWhenTheContextHasIt(t *testing.T) {
	api := dmpfsqs.NewFakeSQS()
	dlq, err := dmpfsqs.NewDLQ(validConfig(), api, standardChannel())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	contained := dmpfports.Contained{Consumer: "c", Reason: dmpfports.ReasonAttemptsExhausted, Envelope: raw}
	if err := dlq.Quarantine(context.Background(), contained); err != nil {
		t.Fatal(err)
	}
	if err := dlq.Quarantine(attempt.WithContext(context.Background(), 2), contained); err != nil {
		t.Fatal(err)
	}
	if _, has := sent(t, api, 0).MessageAttributes[dmpfsqs.AttrAttempt]; has {
		t.Fatal("a containment without attempt in the context carries dmpf-attempt")
	}
	if v := sent(t, api, 1).MessageAttributes[dmpfsqs.AttrAttempt]; v.StringValue == nil || *v.StringValue != "2" {
		t.Fatalf("dmpf-attempt = %v, want 2 (TRP-52)", v.StringValue)
	}
}

func TestQuarantineBoundsTheValuesThatComeFromOutside(t *testing.T) {
	api := dmpfsqs.NewFakeSQS()
	dlq, err := dmpfsqs.NewDLQ(validConfig(), api, standardChannel())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	long := strings.Repeat("é", dmpfsqs.AttributeValueLimit)
	contained := dmpfports.Contained{Consumer: "c", Reason: dmpfports.ReasonTerminalFailure, Envelope: raw, MessageID: dmpfports.MessageID(long), Error: long}
	if err := dlq.Quarantine(context.Background(), contained); err != nil {
		t.Fatal(err)
	}
	attrs := sent(t, api, 0).MessageAttributes
	for _, k := range []string{dmpfsqs.AttrMessageID, dmpfsqs.AttrError} {
		v := *attrs[k].StringValue
		if len(v) > dmpfsqs.AttributeValueLimit || !utf8.ValidString(v) {
			t.Fatalf("%s = %d bytes (valid utf-8: %v), want at most %d and valid", k, len(v), utf8.ValidString(v), dmpfsqs.AttributeValueLimit)
		}
	}
}
