package ids_test

import (
	"sync"
	"testing"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/ids"
)

// claimIDs mirrors relay.ClaimIDs of dmpf-app structurally: the kit must
// satisfy it without importing dmpf-app.
type claimIDs interface{ NewClaimID() string }

var (
	_ dmpfports.IDGenerator = (*ids.Sequence)(nil)
	_ dmpfports.IDGenerator = (*ids.Seeded)(nil)
	_ claimIDs              = (*ids.ClaimIDs)(nil)
)

func TestSequenceIssuesPrefixAndSixDigitCounter(t *testing.T) {
	g := &ids.Sequence{Prefix: "m-"}
	want := []dmpfports.MessageID{"m-000001", "m-000002", "m-000003"}
	for i, w := range want {
		if got := g.NewMessageID(); got != w {
			t.Fatalf("id %d = %q, want %q", i, got, w)
		}
	}
}

func TestSequenceIsUniqueUnderConcurrency(t *testing.T) {
	g := &ids.Sequence{Prefix: "m-"}
	const n = 200
	var wg sync.WaitGroup
	seen := make(chan dmpfports.MessageID, n)
	for range n {
		wg.Go(func() { seen <- g.NewMessageID() })
	}
	wg.Wait()
	close(seen)
	unique := map[dmpfports.MessageID]bool{}
	for id := range seen {
		if unique[id] {
			t.Fatalf("id %q issued twice", id)
		}
		unique[id] = true
	}
	if len(unique) != n {
		t.Fatalf("%d unique ids, want %d", len(unique), n)
	}
}

func TestSeededIsReproducibleAndSeedSensitive(t *testing.T) {
	a := ids.NewSeeded(42)
	b := ids.NewSeeded(42)
	other := ids.NewSeeded(43)
	for i := range 5 {
		x, y, z := a.NewMessageID(), b.NewMessageID(), other.NewMessageID()
		if x != y {
			t.Fatalf("id %d differs for the same seed: %q vs %q", i, x, y)
		}
		if x == z {
			t.Fatalf("id %d equal across seeds: %q", i, x)
		}
		if len(x) != 32 {
			t.Fatalf("id %d = %q, want 32 hex chars", i, x)
		}
	}
}

func TestSeededNeverRepeatsWithinARun(t *testing.T) {
	g := ids.NewSeeded(7)
	seen := map[dmpfports.MessageID]bool{}
	for range 1000 {
		id := g.NewMessageID()
		if seen[id] {
			t.Fatalf("id %q repeated", id)
		}
		seen[id] = true
	}
}

func TestClaimIDsIssueASequence(t *testing.T) {
	c := ids.NewClaimIDs()
	if got := c.NewClaimID(); got != "claim-000001" {
		t.Fatalf("first claim id = %q", got)
	}
	if got := c.NewClaimID(); got != "claim-000002" {
		t.Fatalf("second claim id = %q", got)
	}
}
