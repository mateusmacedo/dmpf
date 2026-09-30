package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestFindReservationReadsOutsideTheUnitOfWork(t *testing.T) {
	h := newSyncHarness(t)
	h.seed(t, confirmedSnapshot(3), 0)

	snapshot, err := h.service.FindReservation(withExecution(t, context.Background()), syncOrder)

	if err != nil {
		t.Fatalf("FindReservation() error = %v, want nil", err)
	}
	if snapshot.Status != domain.Confirmed || snapshot.Items != 3 {
		t.Fatalf("snapshot = %+v, want confirmed with 3 items", snapshot)
	}
	if got := h.serviceWithinCalls(); got != 0 {
		t.Fatalf("transactions opened = %d, want 0 — a query never opens one (UOW-11)", got)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty (UOW-11)", got)
	}
}

func TestFindReservationReportsErrNotFoundForAnAbsentAggregate(t *testing.T) {
	h := newSyncHarness(t)

	_, err := h.service.FindReservation(withExecution(t, context.Background()), "P-404")

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("FindReservation() error = %v, want ErrNotFound through the wrapping", err)
	}
}
