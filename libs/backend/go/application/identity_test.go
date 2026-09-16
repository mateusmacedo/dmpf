package application_test

import (
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type countingClock struct {
	at    ports.Instant
	calls int
}

func (c *countingClock) Now() ports.Instant {
	c.calls++
	return c.at
}

type countingIDs struct {
	prefix string
	calls  int
}

func (g *countingIDs) NewMessageID() ports.MessageID {
	g.calls++
	return ports.MessageID(fmt.Sprintf("%s%06d", g.prefix, g.calls))
}

type fixedIDs struct{ id ports.MessageID }

func (g fixedIDs) NewMessageID() ports.MessageID { return g.id }

func TestResolveIdentityReadsTheClockOnceAndTakesOneIDPerEvent(t *testing.T) {
	clock := &countingClock{at: 1_755_432_000_000_000_000}
	ids := &countingIDs{prefix: "m-"}

	id := application.ResolveIdentity(clock, ids, 2)

	if clock.calls != 1 {
		t.Fatalf("clock read %d times, want 1 — one instant per use case execution", clock.calls)
	}
	if ids.calls != 2 {
		t.Fatalf("took %d identifiers, want 2", ids.calls)
	}
	if id.OccurredAt != clock.at {
		t.Fatalf("OccurredAt = %d, want %d", id.OccurredAt, clock.at)
	}
	if got := id.MessageIDs; len(got) != 2 || got[0] == got[1] {
		t.Fatalf("MessageIDs = %v, want two distinct identifiers", got)
	}
}

func TestResolveIdentityStillReadsTheClockWithoutEvents(t *testing.T) {
	clock := &countingClock{at: 42}
	ids := &countingIDs{prefix: "m-"}

	id := application.ResolveIdentity(clock, ids, 0)

	if clock.calls != 1 {
		t.Fatalf("clock read %d times, want 1 — occurred_at exists even with no event", clock.calls)
	}
	if ids.calls != 0 {
		t.Fatalf("took %d identifiers, want 0", ids.calls)
	}
	if id.MessageIDs == nil {
		t.Fatal("MessageIDs is nil, want an empty sequence")
	}
	if len(id.MessageIDs) != 0 {
		t.Fatalf("MessageIDs = %v, want empty", id.MessageIDs)
	}
}

func TestResolveIdentityPanicsOnAnEmptyIdentifier(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("an empty MessageID must panic: it is a provider defect, not a use case outcome")
		}
	}()

	_ = application.ResolveIdentity(&countingClock{}, fixedIDs{id: ""}, 1)
}

func TestResolveIdentityPanicsOnARepeatedIdentifier(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a repeated MessageID must panic: two facts would share one identity")
		}
	}()

	_ = application.ResolveIdentity(&countingClock{}, fixedIDs{id: "m-000001"}, 2)
}

func TestResolveIdentityPanicsOnANegativeEventCount(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a negative event count must panic: it is a programming defect")
		}
	}()

	_ = application.ResolveIdentity(&countingClock{}, &countingIDs{prefix: "m-"}, -1)
}
