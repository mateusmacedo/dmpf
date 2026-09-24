package providerkit_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// memStore is a drain-side outbox in memory that honours OBX-09..OBX-11 the
// way the Postgres statements do, so the suite has a conforming candidate that
// needs no database; lenient is the fixture that accepts a stale claim.
type memStore struct {
	mu      sync.Mutex
	clock   ports.Clock
	rows    map[int64]*memRow
	next    int64
	lenient bool
}

type memRow struct {
	status      string
	lockedBy    string
	lockedUntil ports.Instant
	availableAt ports.Instant
	attempts    int
	publishedAt ports.Instant
	lastError   string
}

type claimed struct{ id int64 }

var errNoRow = errors.New("memStore: no such row")

func (s *memStore) enqueue(n int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for range n {
		s.next++
		s.rows[s.next] = &memRow{status: "pending", availableAt: s.clock.Now()}
	}
	return nil
}

func (s *memStore) Claim(_ context.Context, claimID string, limit int, lease time.Duration) ([]claimed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	var out []claimed
	for id := int64(1); id <= s.next && len(out) < limit; id++ {
		r, ok := s.rows[id]
		if !ok {
			continue
		}
		eligible := r.availableAt <= now && (r.status == "pending" || (r.status == "publishing" && (r.lockedUntil == 0 || r.lockedUntil <= now)))
		if !eligible {
			continue
		}
		r.status, r.lockedBy, r.lockedUntil = "publishing", claimID, now+ports.Instant(lease)
		r.attempts++
		out = append(out, claimed{id: id})
	}
	return out, nil
}

func (s *memStore) transition(id int64, claimID string, apply func(*memRow)) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[id]
	if !ok {
		return 0, errNoRow
	}
	if r.lockedBy != claimID && !s.lenient {
		return 0, nil
	}
	apply(r)
	return 1, nil
}

func (s *memStore) MarkPublished(_ context.Context, id int64, claimID string) (int64, error) {
	now := s.clock.Now()
	return s.transition(id, claimID, func(r *memRow) { r.status, r.lockedUntil, r.publishedAt, r.lastError = "published", 0, now, "" })
}

func (s *memStore) Reschedule(_ context.Context, id int64, claimID string, availableAt ports.Instant, lastError string) (int64, error) {
	return s.transition(id, claimID, func(r *memRow) {
		r.availableAt, r.lockedUntil = availableAt, 0
		if lastError != "" {
			r.lastError = lastError
		}
	})
}

func (s *memStore) Fail(_ context.Context, id int64, claimID string, lastError string) (int64, error) {
	return s.transition(id, claimID, func(r *memRow) { r.status, r.lockedUntil, r.lastError = "failed", 0, lastError })
}

func (s *memStore) state(id int64) (providerkit.RecordState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[id]
	if !ok {
		return providerkit.RecordState{}, errNoRow
	}
	return providerkit.RecordState{
		Status: r.status, LockedBy: r.lockedBy, LockedUntil: r.lockedUntil, AvailableAt: r.availableAt,
		Attempts: r.attempts, PublishedAt: r.publishedAt, LastError: r.lastError,
	}, nil
}

func (s *memStore) purge(before ports.Instant) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for id, r := range s.rows {
		if r.status == "published" && r.publishedAt < before {
			delete(s.rows, id)
			n++
		}
	}
	return n, nil
}

func (s *memStore) pending() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, r := range s.rows {
		if r.status == "pending" || r.status == "publishing" {
			n++
		}
	}
	return n, nil
}

func memOutbox(lenient bool) func() providerkit.OutboxSubject[claimed] {
	return func() providerkit.OutboxSubject[claimed] {
		c := clock.New(1_000_000_000)
		s := &memStore{clock: c, rows: map[int64]*memRow{}, lenient: lenient}
		return providerkit.OutboxSubject[claimed]{
			Store:   s,
			Enqueue: s.enqueue,
			ID:      func(c claimed) int64 { return c.id },
			Clock:   c,
			State:   s.state,
			Pending: s.pending,
			Purge:   s.purge,
		}
	}
}

func TestAConformingOutboxStorePasses(t *testing.T) {
	v := providerkit.Outbox(memOutbox(false))
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped: %v", v.Skipped)
	}
}

// The negative vector: a store that lets a stale claim transition the record.
func TestALenientStoreIsReprovedOnOBX11(t *testing.T) {
	v := providerkit.Outbox(memOutbox(true))
	if v.OK() {
		t.Fatal("a store that accepts a replaced claim passed")
	}
	for _, d := range v.Diagnostics {
		if d.Rule != "OBX-11" {
			t.Fatalf("unexpected diagnostic: %v", d)
		}
	}
	if len(v.Diagnostics) == 0 {
		t.Fatal("no OBX-11 diagnostic")
	}
}
