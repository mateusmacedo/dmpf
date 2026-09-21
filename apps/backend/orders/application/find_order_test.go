package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestFindOrderReadsOutsideTheUnitOfWork(t *testing.T) {
	h := newHarness(t)
	h.seed(t, openSnapshot(2), 0)

	snapshot, err := h.service.FindOrder(context.Background(), testExecution(t), orderID)

	if err != nil {
		t.Fatalf("FindOrder() error = %v, want nil", err)
	}
	if len(snapshot.Items) != 2 {
		t.Fatalf("snapshot has %d items, want 2", len(snapshot.Items))
	}
	if got := h.serviceWithinCalls(); got != 0 {
		t.Fatalf("transactions opened = %d, want 0 — a query never opens one (UOW-11)", got)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty — a query never writes the outbox (UOW-11)", got)
	}
}

func TestFindOrderReportsErrNotFoundForAnAbsentAggregate(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.FindOrder(context.Background(), testExecution(t), "P-404")

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("FindOrder() error = %v, want ErrNotFound through the wrapping", err)
	}
}
