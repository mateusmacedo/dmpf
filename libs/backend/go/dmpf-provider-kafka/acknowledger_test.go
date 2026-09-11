package dmpfkafka_test

import (
	"context"
	"errors"
	"testing"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfkafka "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-kafka"
)

var _ dmpfports.Acknowledger = (*dmpfkafka.Acknowledger)(nil)

func TestAcknowledgerRecordsOneTerminalGesture(t *testing.T) {
	ctx := context.Background()

	t.Run("ack", func(t *testing.T) {
		var a dmpfkafka.Acknowledger
		if a.Decision() != dmpfkafka.GestureUndecided {
			t.Fatal("a fresh acknowledger is already decided")
		}
		if err := a.Ack(ctx); err != nil {
			t.Fatalf("Ack() = %v", err)
		}
		if a.Decision() != dmpfkafka.GestureAcked {
			t.Fatal("Ack() did not record the gesture")
		}
		if err := a.Release(ctx); !errors.Is(err, dmpfkafka.ErrAlreadyDisposed) {
			t.Fatalf("Release() after Ack() = %v, want ErrAlreadyDisposed (TRP-27)", err)
		}
		if err := a.Ack(ctx); !errors.Is(err, dmpfkafka.ErrAlreadyDisposed) {
			t.Fatalf("second Ack() = %v, want ErrAlreadyDisposed", err)
		}
	})

	t.Run("release", func(t *testing.T) {
		var a dmpfkafka.Acknowledger
		if err := a.Release(ctx); err != nil {
			t.Fatalf("Release() = %v", err)
		}
		if a.Decision() != dmpfkafka.GestureReleased {
			t.Fatal("Release() did not record the gesture")
		}
		if err := a.Ack(ctx); !errors.Is(err, dmpfkafka.ErrAlreadyDisposed) {
			t.Fatalf("Ack() after Release() = %v, want ErrAlreadyDisposed", err)
		}
	})
}
