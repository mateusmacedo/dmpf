package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
)

var nineSteps = []string{
	"authorize",
	"clock.Now",
	"ids.NewMessageID",
	"within",
	"domain.Load",
	"domain.Save",
	"outbox.Enqueue",
	"commit",
}

func TestReserveWalksTheNineStepsInOrder(t *testing.T) {
	h := newSyncHarness(t)

	if _, err := h.service.Reserve(withExecution(t, context.Background()), application.Reserve{Order: syncOrder, Items: 1}); err != nil {
		t.Fatalf("Reserve() error = %v, want nil", err)
	}

	if !slices.Equal(h.rec.observed, nineSteps) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.observed, nineSteps)
	}
}

func TestCancelWalksTheNineStepsInOrder(t *testing.T) {
	h := newSyncHarness(t)

	if _, err := h.service.Cancel(withExecution(t, context.Background()), application.Cancel{Order: syncOrder}); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}

	if !slices.Equal(h.rec.observed, nineSteps) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.observed, nineSteps)
	}
}

func TestADeniedReserveStopsBeforeIdentityAndTransaction(t *testing.T) {
	deny := func(context.Context, application.Operation) error { return ports.ErrDenied }
	h := newSyncHarness(t, withSyncAuthorize(deny))

	_, err := h.service.Reserve(withExecution(t, context.Background()), application.Reserve{Order: syncOrder, Items: 1})

	if !errors.Is(err, ports.ErrDenied) {
		t.Fatalf("Reserve() error = %v, want ErrDenied", err)
	}
	if want := []string{"authorize"}; !slices.Equal(h.rec.observed, want) {
		t.Fatalf("observed %v, want %v", h.rec.observed, want)
	}
	if got := h.store.WithinCalls(); got != 0 {
		t.Fatalf("WithinCalls() = %d, want 0", got)
	}
}
