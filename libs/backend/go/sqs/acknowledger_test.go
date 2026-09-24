package sqs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
)

var _ ports.Acknowledger = (*provider.Acknowledger)(nil)

func TestAckDeletesByReceiptHandleAfterStoppingTheHeartbeat(t *testing.T) {
	api := provider.NewFakeSQS()
	var order []string
	stop := func() { order = append(order, "stop") }
	ack := provider.NewAcknowledger(api, fifoURL, "rh-1", 5*time.Second, stop)

	if err := ack.Ack(context.Background()); err != nil {
		t.Fatalf("Ack() = %v", err)
	}
	deletes := api.Calls("DeleteMessage")
	if len(deletes) != 1 || *deletes[0].Input.(*sqs.DeleteMessageInput).ReceiptHandle != "rh-1" {
		t.Fatalf("DeleteMessage calls = %v, want one with the receipt (SQS-09)", deletes)
	}
	if len(order) != 1 || order[0] != "stop" {
		t.Fatal("the heartbeat was not stopped before the gesture (SQS-08)")
	}
	if len(api.Calls("ChangeMessageVisibility")) != 0 {
		t.Fatal("Ack changed the visibility")
	}
	if err := ack.Release(context.Background()); !errors.Is(err, provider.ErrAlreadyDisposed) {
		t.Fatalf("Release() after Ack() = %v, want ErrAlreadyDisposed (TRP-27)", err)
	}
}

func TestReleaseShortensTheVisibilityAndNeverDeletes(t *testing.T) {
	api := provider.NewFakeSQS()
	ack := provider.NewAcknowledger(api, fifoURL, "rh-2", 2500*time.Millisecond, nil)

	if err := ack.Release(context.Background()); err != nil {
		t.Fatalf("Release() = %v", err)
	}
	changes := api.Calls("ChangeMessageVisibility")
	if len(changes) != 1 {
		t.Fatalf("ChangeMessageVisibility calls = %d, want 1", len(changes))
	}
	in := changes[0].Input.(*sqs.ChangeMessageVisibilityInput)
	if *in.ReceiptHandle != "rh-2" || in.VisibilityTimeout != 3 {
		t.Fatalf("input = %+v, want receipt rh-2 and 3s (2.5s rounded up)", in)
	}
	if len(api.Calls("DeleteMessage")) != 0 {
		t.Fatal("Release deleted the message (SQS-10)")
	}
	if err := ack.Ack(context.Background()); !errors.Is(err, provider.ErrAlreadyDisposed) {
		t.Fatalf("Ack() after Release() = %v, want ErrAlreadyDisposed", err)
	}
}

func TestVisibilitySecondsIsBoundedByTheCeiling(t *testing.T) {
	if got := provider.VisibilitySeconds(0); got != 0 {
		t.Fatalf("0 → %d", got)
	}
	if got := provider.VisibilitySeconds(-time.Second); got != 0 {
		t.Fatalf("negative → %d", got)
	}
	if got := provider.VisibilitySeconds(13 * time.Hour); got != provider.MaxVisibilitySeconds {
		t.Fatalf("13h → %d, want the 12h ceiling (SQS-08b)", got)
	}
}
