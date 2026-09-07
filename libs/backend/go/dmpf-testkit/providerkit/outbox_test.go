package providerkit_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/providerkit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
)

// memStore is a drain-side outbox in memory that honours OBX-09..OBX-11 the
// way the Postgres statements do, so the suite has a conforming candidate that
// needs no database; lenient is the fixture that accepts a stale claim.
type memStore struct {
	mu      sync.Mutex
	clock   dmpfports.Clock
	rows    map[int64]*memRow
	next    int64
	lenient bool
}

type memRow struct {
	status      string
	lockedBy    string
	lockedUntil dmpfports.Instant
	availableAt dmpfports.Instant
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
		r := s.rows[id]
		eligible := r.availableAt <= now && (r.status == "pending" || (r.status == "publishing" && (r.lockedUntil == 0 || r.lockedUntil <= now)))
		if !eligible {
			continue
		}
		r.status, r.lockedBy, r.lockedUntil = "publishing", claimID, now+dmpfports.Instant(lease)
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
	return s.transition(id, claimID, func(r *memRow) { r.status, r.lockedUntil = "published", 0 })
}

func (s *memStore) Reschedule(_ context.Context, id int64, claimID string, availableAt dmpfports.Instant, _ string) (int64, error) {
	return s.transition(id, claimID, func(r *memRow) { r.availableAt, r.lockedUntil = availableAt, 0 })
}

func (s *memStore) Fail(_ context.Context, id int64, claimID string, _ string) (int64, error) {
	return s.transition(id, claimID, func(r *memRow) { r.status, r.lockedUntil = "failed", 0 })
}

func (s *memStore) status(id int64) (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[id]
	if !ok {
		return "", "", errNoRow
	}
	return r.status, r.lockedBy, nil
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
			Status:  s.status,
			Pending: s.pending,
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
