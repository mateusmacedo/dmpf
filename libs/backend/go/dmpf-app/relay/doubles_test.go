package relay

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

// transition is one write the loop asked the store for, kept so a test can
// assert which outcome ran and with which claim.
type transition struct {
	kind        string
	id          int64
	claimID     string
	availableAt dmpfports.Instant
	lastError   string
}

// fakeStore hands out scripted batches and records every transition. affected
// is what the three transitions report; zero is how a test says the claim was
// replaced (OBX-10).
type fakeStore struct {
	mu          sync.Mutex
	batches     [][]dmpfpostgres.Claimed
	claimIDs    []string
	transitions []transition
	affected    int64
	claims      atomic.Int64
	health      dmpfpostgres.OutboxHealth

	// failOnCancelledContext makes the doubles behave like the real store,
	// where a write on a dead context never reaches the database.
	failOnCancelledContext bool
	cancelledWrites        atomic.Int64

	// failMarkPublished is the failure window of FND-04 §5.4: the message
	// reached the broker and the transition did not land.
	failMarkPublished bool
}

func newFakeStore(batches ...[]dmpfpostgres.Claimed) *fakeStore {
	return &fakeStore{batches: batches, affected: 1}
}

func (s *fakeStore) Claim(_ context.Context, claimID string, _ int, _ time.Duration) ([]dmpfpostgres.Claimed, error) {
	s.claims.Add(1)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimIDs = append(s.claimIDs, claimID)
	if len(s.batches) == 0 {
		return nil, nil
	}
	batch := s.batches[0]
	s.batches = s.batches[1:]
	for i := range batch {
		batch[i].LockedBy = claimID
	}
	return batch, nil
}

func (s *fakeStore) MarkPublished(ctx context.Context, id int64, claimID string) (int64, error) {
	if s.failMarkPublished {
		return 0, errTransitionLost
	}
	return s.record(ctx, transition{kind: "published", id: id, claimID: claimID})
}

func (s *fakeStore) Reschedule(ctx context.Context, id int64, claimID string, availableAt dmpfports.Instant, lastError string) (int64, error) {
	return s.record(ctx, transition{kind: "rescheduled", id: id, claimID: claimID, availableAt: availableAt, lastError: lastError})
}

func (s *fakeStore) Fail(ctx context.Context, id int64, claimID string, lastError string) (int64, error) {
	return s.record(ctx, transition{kind: "failed", id: id, claimID: claimID, lastError: lastError})
}

func (s *fakeStore) OutboxSignals(context.Context) (dmpfpostgres.OutboxHealth, error) {
	return s.health, nil
}

func (s *fakeStore) record(ctx context.Context, t transition) (int64, error) {
	if s.failOnCancelledContext && ctx.Err() != nil {
		s.cancelledWrites.Add(1)
		return 0, ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.transitions = append(s.transitions, t)
	return s.affected, nil
}

func (s *fakeStore) recorded() []transition {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]transition(nil), s.transitions...)
}

// fakePublisher counts deliveries and can fail on demand. inFlight tracks
// concurrent calls so the concurrency limit can be asserted.
type fakePublisher struct {
	err       error
	delivered atomic.Int64
	inFlight  atomic.Int64
	peak      atomic.Int64
	hold      time.Duration
}

func (p *fakePublisher) Publish(context.Context, string, []byte) error {
	current := p.inFlight.Add(1)
	for {
		peak := p.peak.Load()
		if current <= peak || p.peak.CompareAndSwap(peak, current) {
			break
		}
	}
	if p.hold > 0 {
		time.Sleep(p.hold)
	}
	p.inFlight.Add(-1)
	p.delivered.Add(1)
	return p.err
}

// countingIDs is the ClaimIDs port: OBX-08 asks only that two acquisitions
// never share an identity, which a counter satisfies deterministically.
type countingIDs struct{ n atomic.Int64 }

func (c *countingIDs) NewClaimID() string {
	return "claim-" + time.Duration(c.n.Add(1)).String()
}

type fixedClock dmpfports.Instant

func (c fixedClock) Now() dmpfports.Instant { return dmpfports.Instant(c) }

func batchOf(t testingT, n int) []dmpfpostgres.Claimed {
	t.Helper()

	batch := make([]dmpfpostgres.Claimed, 0, n)
	for i := range n {
		record := publishableRecord(t)
		record.ID = int64(i + 1)
		batch = append(batch, record)
	}
	return batch
}

// testingT is the slice of *testing.T these helpers need, so the fixture
// builder can be shared without importing testing into non-test code.
type testingT interface {
	Helper()
	Fatalf(format string, args ...any)
}
