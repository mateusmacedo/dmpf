package dmpfsqs_test

import (
	"bytes"
	"errors"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/payloadhash"
	dmpfsqs "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-sqs"
)

func validRaw(t *testing.T, partitionKey string) ([]byte, envelope.Envelope) {
	t.Helper()
	env := envelope.Envelope{
		ID: "evt-1", Source: "urn:dmpf:stock", SpecVersion: envelope.SpecVersion,
		Type: "stock.reservation.requested.v1", Subject: "reservation/" + partitionKey, Time: timestamppb.New(start),
		DataSchema: "type.googleapis.com/stock.v1.ReservationRequested", DataContentType: envelope.ContentType,
		CorrelationID: "corr-1", CausationID: "evt-0", PartitionKey: partitionKey,
		TraceParent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		Payload:     []byte{0x0a, 0x03, 'r', '-', '1', 0x10, 0x02},
	}
	raw, err := envelope.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return raw, env
}

func TestBodyRoundTripIsASingleEncoding(t *testing.T) {
	raw, env := validRaw(t, "k1")
	body := dmpfsqs.EncodeBody(raw)

	decoded, err := dmpfsqs.DecodeBody(body)
	if err != nil {
		t.Fatalf("DecodeBody() = %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatal("decoded body differs from the envelope (SQS-01)")
	}
	got, err := envelope.Unmarshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if payloadhash.Sum(got.Payload) != payloadhash.Sum(env.Payload) {
		t.Fatal("payload hash differs after the textual hop (TRP-15)")
	}
}

func TestDoubleBase64DivergesTheHash(t *testing.T) {
	raw, env := validRaw(t, "k1")
	twice := dmpfsqs.EncodeBody([]byte(dmpfsqs.EncodeBody(raw)))

	once, err := dmpfsqs.DecodeBody(twice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := envelope.Unmarshal(once); err == nil {
		t.Fatal("a doubly encoded body still decoded as the envelope: the negative vector proves nothing")
	}
	if payloadhash.Sum(once) == payloadhash.Sum(env.Payload) {
		t.Fatal("the hash of the doubly encoded body matches the payload's (TRP-19 negative)")
	}
}

func TestDecodeBodyRefusesTheSNSWrapper(t *testing.T) {
	wrapper := `{"Type":"Notification","MessageId":"f29a","TopicArn":"arn:aws:sns:us-east-1:000000000000:t","Message":"cmF3","Timestamp":"2026-09-06T17:00:00Z"}`
	if _, err := dmpfsqs.DecodeBody(wrapper); !errors.Is(err, dmpfsqs.ErrSNSEnvelopeNotRaw) {
		t.Fatalf("DecodeBody(wrapper) = %v, want ErrSNSEnvelopeNotRaw (SQS-02)", err)
	}
	if _, err := dmpfsqs.DecodeBody(`{"Type":"Other"}`); !errors.Is(err, dmpfsqs.ErrInvalidBody) {
		t.Fatalf("DecodeBody(other json) = %v, want ErrInvalidBody", err)
	}
	if _, err := dmpfsqs.DecodeBody("not base64!"); !errors.Is(err, dmpfsqs.ErrInvalidBody) {
		t.Fatalf("DecodeBody(garbage) = %v, want ErrInvalidBody", err)
	}
}
