package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestReserveCreatesTheReservationAndAuthorsTheOutboxEntry(t *testing.T) {
	h := newSyncHarness(t)

	out, err := h.service.Reserve(withExecution(t, context.Background()), application.Reserve{Order: syncOrder, Items: 2})

	if err != nil {
		t.Fatalf("Reserve() error = %v, want nil", err)
	}
	if rej, refused := out.Rejection(); refused {
		t.Fatalf("Rejection() = %v, want no refusal", rej)
	}
	if got, want := out.Response(), (domain.ReservedResponse{Order: syncOrder, Items: 2}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}

	snapshot, version, err := reservationTable.Reader(h.store).Load(withExecution(t, context.Background()), syncOrder)
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if version != 1 || snapshot.Status != domain.Confirmed || snapshot.Items != 2 {
		t.Fatalf("stored = %+v v%d, want confirmed with 2 items at v1", snapshot, version)
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
		Event:            domain.ReservationConfirmed{Order: syncOrder, Items: 2, At: domain.Instant(syncOccurred)},
		Context:          ports.MessageContext{CausationID: "m-000001"},
	}
	if entries[0] != want {
		t.Fatalf("OutboxEntry mismatch\ngot:  %+v\nwant: %+v", entries[0], want)
	}
}

func TestReserveCopiesTheMessageContextIntoTheOutboxEntry(t *testing.T) {
	h := newSyncHarness(t)
	const traceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	ctx := ports.WithMessageContext(context.Background(), ports.MessageContext{
		CorrelationID: "corr-1", CausationID: "req-ctx-1", Traceparent: traceparent,
	})

	if _, err := h.service.Reserve(withExecution(t, ctx), application.Reserve{Order: syncOrder, Items: 1}); err != nil {
		t.Fatalf("Reserve() error = %v, want nil", err)
	}

	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() has %d entries, want 1", len(entries))
	}
	want := ports.MessageContext{CorrelationID: "corr-1", CausationID: "req-ctx-1", Traceparent: traceparent}
	if entries[0].Context != want {
		t.Fatalf("Context = %+v, want %+v — the edge's causation is kept", entries[0].Context, want)
	}
}

func TestReserveOnACanceledReservationRejectsWithoutWriting(t *testing.T) {
	h := newSyncHarness(t)
	h.seed(t, canceledSnapshot(), 0)

	out, err := h.service.Reserve(withExecution(t, context.Background()), application.Reserve{Order: syncOrder, Items: 1})

	if err != nil {
		t.Fatalf("Reserve() error = %v, want nil — a refusal is not a technical failure (DEC-04)", err)
	}
	rej, refused := out.Rejection()
	if !refused || rej.Code() != domain.CodeReservationCancelled {
		t.Fatalf("Rejection() = %v, %v; want %q", rej, refused, domain.CodeReservationCancelled)
	}
	if _, version, _ := reservationTable.Reader(h.store).Load(withExecution(t, context.Background()), syncOrder); version != 1 {
		t.Fatalf("version = %d, want 1 — a refusal writes nothing", version)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
	if h.serviceWithinCalls() != 1 || h.serviceCommits() != 1 {
		t.Fatalf("within = %d, commits = %d; want 1 and 1 (UOW-05)", h.serviceWithinCalls(), h.serviceCommits())
	}
	if h.saves != 0 || h.enqueues != 0 {
		t.Fatalf("a refusal wrote: saves = %d, enqueues = %d", h.saves, h.enqueues)
	}
}

func TestReserveUnderAVersionConflictStopsBeforeTheOutbox(t *testing.T) {
	h := newSyncHarness(t, withSyncSaveError(ports.ErrVersionConflict))

	_, err := h.service.Reserve(withExecution(t, context.Background()), application.Reserve{Order: syncOrder, Items: 1})

	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Reserve() error = %v, want ErrVersionConflict", err)
	}
	if h.binds != 1 || h.saves != 1 || h.enqueues != 0 {
		t.Fatalf("binds = %d, saves = %d, enqueues = %d; want 1, 1 and 0 (UOW-09, UOW-10)", h.binds, h.saves, h.enqueues)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
}
