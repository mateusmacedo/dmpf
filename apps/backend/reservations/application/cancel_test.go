package application_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

func TestCancelCreatesACanceledReservationAndAuthorsTheOutboxEntry(t *testing.T) {
	h := newSyncHarness(t)

	out, err := h.service.Cancel(context.Background(), testExecution(t), application.Cancel{Order: syncOrder})

	if err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	if got, want := out.Response(), (domain.CancelledResponse{Order: syncOrder}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}

	snapshot, version, err := reservationsTable.Reader(h.store).Load(context.Background(), syncOrder)
	if err != nil || version != 1 || snapshot.Status != domain.Canceled {
		t.Fatalf("stored = %+v v%d (%v), want canceled at v1", snapshot, version, err)
	}

	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() has %d entries, want 1", len(entries))
	}
	want := ports.OutboxEntry{
		MessageID:        "m-000001",
		OccurredAt:       syncOccurred,
		Intent:           ports.PublishIntent{Destination: application.Destination, PartitionKey: string(syncOrder)},
		AggregateType:    application.AggregateType,
		AggregateID:      string(syncOrder),
		AggregateVersion: 1,
		Event:            domain.ReservationCancelled{Order: syncOrder, At: domain.Instant(syncOccurred.Unix())},
		Context:          ports.MessageContext{CausationID: "m-000001"},
	}
	if entries[0] != want {
		t.Fatalf("OutboxEntry mismatch\ngot:  %+v\nwant: %+v", entries[0], want)
	}
}

func TestCancelOnAConfirmedReservationRejectsWithoutWriting(t *testing.T) {
	h := newSyncHarness(t)
	h.seed(t, confirmedSnapshot(2), 0)

	out, err := h.service.Cancel(context.Background(), testExecution(t), application.Cancel{Order: syncOrder})

	if err != nil {
		t.Fatalf("Cancel() error = %v, want nil — a refusal is not a technical failure (DEC-04)", err)
	}
	rej, refused := out.Rejection()
	if !refused || rej.Code() != domain.CodeAlreadyReserved {
		t.Fatalf("Rejection() = %v, %v; want %q — the first decision won", rej, refused, domain.CodeAlreadyReserved)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
	if h.saves != 0 || h.enqueues != 0 {
		t.Fatalf("a refusal wrote: saves = %d, enqueues = %d", h.saves, h.enqueues)
	}
}
