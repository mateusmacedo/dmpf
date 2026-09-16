package kafka_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var _ ports.Acknowledger = (*kafka.Acknowledger)(nil)

func TestAcknowledgerRecordsOneTerminalGesture(t *testing.T) {
	ctx := context.Background()

	t.Run("ack", func(t *testing.T) {
		var a kafka.Acknowledger
		if a.Decision() != kafka.GestureUndecided {
			t.Fatal("a fresh acknowledger is already decided")
		}
		if err := a.Ack(ctx); err != nil {
			t.Fatalf("Ack() = %v", err)
		}
		if a.Decision() != kafka.GestureAcked {
			t.Fatal("Ack() did not record the gesture")
		}
		if err := a.Release(ctx); !errors.Is(err, kafka.ErrAlreadyDisposed) {
			t.Fatalf("Release() after Ack() = %v, want ErrAlreadyDisposed (TRP-27)", err)
		}
		if err := a.Ack(ctx); !errors.Is(err, kafka.ErrAlreadyDisposed) {
			t.Fatalf("second Ack() = %v, want ErrAlreadyDisposed", err)
		}
	})

	t.Run("release", func(t *testing.T) {
		var a kafka.Acknowledger
		if err := a.Release(ctx); err != nil {
			t.Fatalf("Release() = %v", err)
		}
		if a.Decision() != kafka.GestureReleased {
			t.Fatal("Release() did not record the gesture")
		}
		if err := a.Ack(ctx); !errors.Is(err, kafka.ErrAlreadyDisposed) {
			t.Fatalf("Ack() after Release() = %v, want ErrAlreadyDisposed", err)
		}
	})
}
