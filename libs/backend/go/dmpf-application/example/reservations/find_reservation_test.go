package reservationsapp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/reservations"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

func TestFindReservationReadsOutsideTheUnitOfWork(t *testing.T) {
	h := newSyncHarness(t)
	h.seed(t, confirmedSnapshot(3), 0)

	snapshot, err := h.service.FindReservation(context.Background(), syncOrder)

	if err != nil {
		t.Fatalf("FindReservation() error = %v, want nil", err)
	}
	if snapshot.Status != reservations.Confirmed || snapshot.Items != 3 {
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

	_, err := h.service.FindReservation(context.Background(), "P-404")

	if !errors.Is(err, dmpfports.ErrNotFound) {
		t.Fatalf("FindReservation() error = %v, want ErrNotFound through the wrapping", err)
	}
}
