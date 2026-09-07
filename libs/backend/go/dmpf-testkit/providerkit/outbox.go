package providerkit

import (
	"context"
	"time"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/clock"
)

// OutboxStore is the drain side of the outbox as the relay of dmpf-app declares
// it, restated here structurally: a provider-block unit may not import app.
// Claimed is the generic parameter because its shape is the provider's.
type OutboxStore[C any] interface {
	Claim(ctx context.Context, claimID string, limit int, lease time.Duration) ([]C, error)
	MarkPublished(ctx context.Context, id int64, claimID string) (int64, error)
	Reschedule(ctx context.Context, id int64, claimID string, availableAt dmpfports.Instant, lastError string) (int64, error)
	Fail(ctx context.Context, id int64, claimID string, lastError string) (int64, error)
}

// OutboxSubject is what a realization gives the suite: the store, a way to
// enqueue n pending records, the record identity, the fake clock the store
// reads leases against, and a reader of the committed status of one record.
type OutboxSubject[C any] struct {
	Store   OutboxStore[C]
	Enqueue func(n int) error
	ID      func(c C) int64
	Clock   *clock.Fake
	Status  func(id int64) (status string, lockedBy string, err error)
	Pending func() (int64, error)
}

const (
	claimA = "claim-a"
	claimB = "claim-b"
	lease  = time.Minute
)

// Outbox exercises the claim and the three conditional transitions of FND-04
// §5.4: a claim is a lease (OBX-09), a transition lands only for the current
// claimant (OBX-10), and an expired or replaced claim never mutates the record
// (OBX-11). newSubject must return a fresh, empty outbox on every call.
func Outbox[C any](newSubject func() OutboxSubject[C]) Verdict {
	var v Verdict
	ctx := context.Background()

	claimOne := func(clause string, s OutboxSubject[C], claimID string) (int64, bool) {
		claimed, err := s.Store.Claim(ctx, claimID, 10, lease)
		if err != nil {
			v.fail(clause, "OBX-09", "Claim(%s) = %v, want nil", claimID, err)
			return 0, false
		}
		if len(claimed) != 1 {
			v.fail(clause, "OBX-09", "Claim(%s) returned %d records, want 1", claimID, len(claimed))
			return 0, false
		}
		return s.ID(claimed[0]), true
	}

	{
		const clause = "a claimed record is invisible to a second claim under the lease"
		s := newSubject()
		if err := s.Enqueue(1); err != nil {
			v.fail(clause, "OBX-09", "Enqueue: %v", err)
		} else if _, ok := claimOne(clause, s, claimA); ok {
			again, err := s.Store.Claim(ctx, claimB, 10, lease)
			if err != nil {
				v.fail(clause, "OBX-09", "second Claim = %v, want nil", err)
			} else if len(again) != 0 {
				v.fail(clause, "OBX-09", "second claim saw %d record(s) under a live lease, want 0", len(again))
			}
		}
	}

	{
		const clause = "a record whose lease expired is claimable again"
		s := newSubject()
		if err := s.Enqueue(1); err != nil {
			v.fail(clause, "OBX-09", "Enqueue: %v", err)
		} else if id, ok := claimOne(clause, s, claimA); ok {
			s.Clock.Advance(lease + time.Second)
			again, err := s.Store.Claim(ctx, claimB, 10, lease)
			if err != nil {
				v.fail(clause, "OBX-09", "Claim after the lease = %v, want nil", err)
			} else if len(again) != 1 || s.ID(again[0]) != id {
				v.fail(clause, "OBX-09", "Claim after the lease returned %d record(s), want the same one", len(again))
			}
		}
	}

	{
		const clause = "MarkPublished by the current claimant transitions the record"
		s := newSubject()
		if err := s.Enqueue(1); err != nil {
			v.fail(clause, "OBX-10", "Enqueue: %v", err)
		} else if id, ok := claimOne(clause, s, claimA); ok {
			n, err := s.Store.MarkPublished(ctx, id, claimA)
			if err != nil {
				v.fail(clause, "OBX-10", "MarkPublished = %v, want nil", err)
			} else if n != 1 {
				v.fail(clause, "OBX-10", "MarkPublished affected %d rows, want 1", n)
			} else if status, _, err := s.Status(id); err != nil || status != "published" {
				v.fail(clause, "OBX-10", "status = %q (%v), want published", status, err)
			}
		}
	}

	{
		const clause = "a replaced claim cannot transition the record"
		s := newSubject()
		if err := s.Enqueue(1); err != nil {
			v.fail(clause, "OBX-11", "Enqueue: %v", err)
		} else if id, ok := claimOne(clause, s, claimA); ok {
			s.Clock.Advance(lease + time.Second)
			if _, ok := claimOne(clause, s, claimB); ok {
				for _, tr := range []struct {
					name string
					run  func() (int64, error)
				}{
					{"MarkPublished", func() (int64, error) { return s.Store.MarkPublished(ctx, id, claimA) }},
					{"Reschedule", func() (int64, error) { return s.Store.Reschedule(ctx, id, claimA, s.Clock.Now(), "late") }},
					{"Fail", func() (int64, error) { return s.Store.Fail(ctx, id, claimA, "late") }},
				} {
					n, err := tr.run()
					if err != nil {
						v.fail(clause, "OBX-11", "%s by the replaced claim = %v, want nil with zero rows", tr.name, err)
					} else if n != 0 {
						v.fail(clause, "OBX-11", "%s by the replaced claim affected %d rows, want 0", tr.name, n)
					}
				}
				if status, lockedBy, err := s.Status(id); err != nil || status != "publishing" || lockedBy != claimB {
					v.fail(clause, "OBX-11", "record after the stale transitions = (%q, %q, %v), want (publishing, %s)", status, lockedBy, err, claimB)
				}
			}
		}
	}

	{
		const clause = "Reschedule releases the lease and the record returns at available_at"
		s := newSubject()
		if err := s.Enqueue(1); err != nil {
			v.fail(clause, "OBX-18", "Enqueue: %v", err)
		} else if id, ok := claimOne(clause, s, claimA); ok {
			later := s.Clock.Now() + dmpfports.Instant(30*time.Second)
			if n, err := s.Store.Reschedule(ctx, id, claimA, later, "transient"); err != nil || n != 1 {
				v.fail(clause, "OBX-18", "Reschedule = (%d, %v), want (1, nil)", n, err)
			} else {
				early, err := s.Store.Claim(ctx, claimB, 10, lease)
				if err != nil {
					v.fail(clause, "OBX-18", "Claim before available_at = %v", err)
				} else if len(early) != 0 {
					v.fail(clause, "OBX-18", "a rescheduled record was claimed before available_at")
				}
				s.Clock.Advance(30 * time.Second)
				due, err := s.Store.Claim(ctx, claimB, 10, lease)
				if err != nil || len(due) != 1 {
					v.fail(clause, "OBX-18", "Claim at available_at = (%d, %v), want the record back", len(due), err)
				}
			}
		}
	}

	{
		const clause = "Fail is terminal for the automatic cycle"
		s := newSubject()
		if err := s.Enqueue(1); err != nil {
			v.fail(clause, "OBX-06", "Enqueue: %v", err)
		} else if id, ok := claimOne(clause, s, claimA); ok {
			if n, err := s.Store.Fail(ctx, id, claimA, "gave up"); err != nil || n != 1 {
				v.fail(clause, "OBX-06", "Fail = (%d, %v), want (1, nil)", n, err)
			} else {
				s.Clock.Advance(lease + time.Second)
				if again, err := s.Store.Claim(ctx, claimB, 10, lease); err != nil || len(again) != 0 {
					v.fail(clause, "OBX-06", "a failed record was claimed again (%d, %v)", len(again), err)
				}
				if status, _, err := s.Status(id); err != nil || status != "failed" {
					v.fail(clause, "OBX-06", "status = %q (%v), want failed", status, err)
				}
			}
		}
	}

	{
		const clause = "Pending counts what is still owed to a broker"
		s := newSubject()
		if s.Pending == nil {
			v.skip(clause)
		} else if err := s.Enqueue(2); err != nil {
			v.fail(clause, "OBX-12", "Enqueue: %v", err)
		} else if first, err := s.Store.Claim(ctx, claimA, 1, lease); err != nil || len(first) != 1 {
			v.fail(clause, "OBX-12", "Claim(limit 1) = (%d, %v), want one record", len(first), err)
		} else {
			// One of the two is published; the other stays owed.
			if _, err := s.Store.MarkPublished(ctx, s.ID(first[0]), claimA); err != nil {
				v.fail(clause, "OBX-12", "MarkPublished = %v", err)
			} else if pending, err := s.Pending(); err != nil || pending != 1 {
				v.fail(clause, "OBX-12", "Pending = (%d, %v), want 1", pending, err)
			}
		}
	}

	return v
}
