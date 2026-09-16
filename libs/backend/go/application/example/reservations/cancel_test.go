package reservationsapp_test

import (
	"context"
	"testing"

	reservationsapp "github.com/mateusmacedo/dmpf/libs/backend/go/application/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestCancelCreatesACanceledReservationAndAuthorsTheOutboxEntry(t *testing.T) {
	h := newSyncHarness(t)

	out, err := h.service.Cancel(context.Background(), reservationsapp.Cancel{Order: syncOrder})

	if err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	if got, want := out.Response(), (reservations.CancelledResponse{Order: syncOrder}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}

	snapshot, version, err := h.store.ReservationsReader().Load(context.Background(), syncOrder)
	if err != nil || version != 1 || snapshot.Status != reservations.Canceled {
		t.Fatalf("stored = %+v v%d (%v), want canceled at v1", snapshot, version, err)
	}

	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() has %d entries, want 1", len(entries))
	}
	want := ports.OutboxEntry{
		MessageID:        "m-000001",
		OccurredAt:       syncOccurred,
		Intent:           ports.PublishIntent{Destination: reservationsapp.Destination, PartitionKey: string(syncOrder)},
		AggregateType:    reservationsapp.AggregateType,
		AggregateID:      string(syncOrder),
		AggregateVersion: 1,
		Event:            reservations.ReservationCancelled{Order: syncOrder, At: reservations.Instant(syncOccurred.Unix())},
		Context:          ports.MessageContext{CausationID: "m-000001"},
	}
	if entries[0] != want {
		t.Fatalf("OutboxEntry mismatch\ngot:  %+v\nwant: %+v", entries[0], want)
	}
}

func TestCancelOnAConfirmedReservationRejectsWithoutWriting(t *testing.T) {
	h := newSyncHarness(t)
	h.seed(t, confirmedSnapshot(2), 0)

	out, err := h.service.Cancel(context.Background(), reservationsapp.Cancel{Order: syncOrder})

	if err != nil {
		t.Fatalf("Cancel() error = %v, want nil — a refusal is not a technical failure (DEC-04)", err)
	}
	rej, refused := out.Rejection()
	if !refused || rej.Code() != reservations.CodeAlreadyReserved {
		t.Fatalf("Rejection() = %v, %v; want %q — the first decision won", rej, refused, reservations.CodeAlreadyReserved)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
	if h.saves != 0 || h.enqueues != 0 {
		t.Fatalf("a refusal wrote: saves = %d, enqueues = %d", h.saves, h.enqueues)
	}
}
