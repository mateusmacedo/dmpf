//go:build integration

package appkit_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/appkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
)

const at = ports.Instant(1_757_000_000_000_000_000)

func newHarness(t *testing.T) appkit.Harness {
	t.Helper()
	return appkit.NewReservations(t, clock.New(at), &ids.Sequence{Prefix: "m-"})
}

func TestFirstReceptionShowsTheOutcomeAtTheEffectEdge(t *testing.T) {
	h := newHarness(t)
	outcome, ack, err := h.Deliver(context.Background(), "evt-1", appkit.RawOrderPlaced(t, "evt-1", "o-1", 2), 1)
	if err != nil || outcome.Disposition != application.R1D1 || outcome.Contained {
		t.Fatalf("outcome = %+v, err = %v", outcome, err)
	}
	if got := h.Effects(t); got != (appkit.Effects{Inbox: 1, Reservations: 1, Outbox: 1}) {
		t.Fatalf("effects = %+v (INB-07)", got)
	}
	if ack.Acks != 1 || ack.Releases != 0 || ack.InboxAtAck != 1 {
		t.Fatalf("ack=%d release=%d inboxAtAck=%d: the ack must follow the commit (INB-08)", ack.Acks, ack.Releases, ack.InboxAtAck)
	}
}

func TestRejectedShowsNothingButTheInboxRow(t *testing.T) {
	h := newHarness(t)
	outcome, _, err := h.Deliver(context.Background(), "evt-2", appkit.RawOrderPlaced(t, "evt-2", "o-2", 0), 1)
	if err != nil || outcome.Disposition != application.R1D2 {
		t.Fatalf("outcome = %+v, err = %v", outcome, err)
	}
	if got := h.Effects(t); got != (appkit.Effects{Inbox: 1}) {
		t.Fatalf("effects = %+v, want only the inbox row", got)
	}
	if status, lastError := h.InboxRow(t, "evt-2"); status != "rejected" || lastError == nil || *lastError != string(reservations.CodeNothingToReserve) {
		t.Fatalf("inbox = (%s, %v)", status, lastError)
	}
}

// V32 in process: a redelivery with a new message_id is classified R1 and only
// the natural key of the effect keeps the reservation from doubling (GAR-10).
func TestRedeliveryWithANewMessageIDDoesNotDuplicateTheEffect(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, _, err := h.Deliver(ctx, "evt-1", appkit.RawOrderPlaced(t, "evt-1", "o-1", 2), 1); err != nil {
		t.Fatalf("first: %v", err)
	}
	outcome, ack, err := h.Deliver(ctx, "evt-9", appkit.RawOrderPlaced(t, "evt-9", "o-1", 2), 1)
	if err != nil || outcome.Disposition != application.R1D2 {
		t.Fatalf("outcome = %+v, err = %v, want R1×D2 from the already-reserved rule", outcome, err)
	}
	if got := h.Effects(t); got != (appkit.Effects{Inbox: 2, Reservations: 1, Outbox: 1}) {
		t.Fatalf("effects = %+v, want two inbox rows and still one reservation and one event", got)
	}
	if status, lastError := h.InboxRow(t, "evt-9"); status != "rejected" || lastError == nil || *lastError != string(reservations.CodeAlreadyReserved) {
		t.Fatalf("inbox evt-9 = (%s, %v)", status, lastError)
	}
	if ack.Acks != 1 {
		t.Fatalf("ack=%d", ack.Acks)
	}
}

func TestInvalidBytesAreContainedWithoutTouchingTheInbox(t *testing.T) {
	h := newHarness(t)
	outcome, ack, err := h.Deliver(context.Background(), "", []byte("definitely not a cloudevent"), 1)
	if err != nil || outcome.Classified || !outcome.Contained || outcome.Reason != ports.ReasonInvalidEnvelope {
		t.Fatalf("outcome = %+v, err = %v", outcome, err)
	}
	if got := h.Effects(t); got != (appkit.Effects{Quarantine: 1}) {
		t.Fatalf("effects = %+v, want only the quarantine row (INB-10)", got)
	}
	if ack.Acks != 1 {
		t.Fatalf("ack=%d", ack.Acks)
	}
}
