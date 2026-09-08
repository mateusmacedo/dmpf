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

// RecordState is the observable state of one outbox record, whole: OBX-11 says
// a replaced claim changes nothing, and "nothing" is every column, not the two
// a transition happens to write.
type RecordState struct {
	Status      string
	LockedBy    string
	LockedUntil dmpfports.Instant
	AvailableAt dmpfports.Instant
	Attempts    int
	PublishedAt dmpfports.Instant
	LastError   string
}

// OutboxSubject is what a realization gives the suite: the store, a way to
// enqueue n pending records, the record identity, the fake clock the store
// reads leases against, the whole state of one record, the pending signal of
// OBX-12 and the purge of OBX-17. None is optional: purge and signals are part
// of the contract KIT-04 certifies.
type OutboxSubject[C any] struct {
	Store   OutboxStore[C]
	Enqueue func(n int) error
	ID      func(c C) int64
	Clock   *clock.Fake
	State   func(id int64) (RecordState, error)
	Pending func() (int64, error)
	Purge   func(before dmpfports.Instant) (removed int64, err error)
}

const (
	claimA = "claim-a"
	claimB = "claim-b"
	lease  = time.Minute
)

// Outbox exercises the claim and the three conditional transitions of FND-04
// §5.4: a claim is a lease (OBX-09), a transition lands only for the current
// claimant (OBX-10), and a replaced claim never mutates the record (OBX-11 —
// substitution, not expiry: an expired claim nobody reacquired may still write).
// newSubject must return a fresh, empty outbox on every call.
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
			} else if st, err := s.State(id); err != nil || st.Status != "published" {
				v.fail(clause, "OBX-10", "status = %q (%v), want published", st.Status, err)
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
				before, err := s.State(id)
				if err != nil {
					v.fail(clause, "OBX-11", "State: %v", err)
				}
				for _, tr := range []struct {
					name string
					run  func() (int64, error)
				}{
					{"MarkPublished", func() (int64, error) { return s.Store.MarkPublished(ctx, id, claimA) }},
					{"Reschedule", func() (int64, error) { return s.Store.Reschedule(ctx, id, claimA, s.Clock.Now(), "late") }},
					{"Fail", func() (int64, error) { return s.Store.Fail(ctx, id, claimA, "late") }},
				} {
					n, err := tr.run()
					switch {
					case err != nil:
						v.fail(clause, "OBX-11", "%s by the replaced claim = %v, want nil with zero rows", tr.name, err)
					case n != 0:
						v.fail(clause, "OBX-11", "%s by the replaced claim affected %d rows, want 0", tr.name, n)
					}
					if after, err := s.State(id); err != nil || after != before {
						v.fail(clause, "OBX-11", "record changed after %s by the replaced claim: %+v → %+v (%v)", tr.name, before, after, err)
						before = after
					}
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
				if st, err := s.State(id); err != nil || st.Status != "failed" {
					v.fail(clause, "OBX-06", "status = %q (%v), want failed", st.Status, err)
				}
			}
		}
	}

	{
		const clause = "Pending counts what is still owed to a broker"
		s := newSubject()
		if s.Pending == nil {
			v.fail(clause, "OBX-12", "the subject declares no Pending signal; the drain-side signals are part of the contract")
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

	{
		const clause = "Purge removes only what was published before the cutoff"
		s := newSubject()
		if s.Purge == nil {
			v.fail(clause, "OBX-17", "the subject declares no Purge; retention is part of the contract")
		} else if err := s.Enqueue(2); err != nil {
			v.fail(clause, "OBX-17", "Enqueue: %v", err)
		} else if first, err := s.Store.Claim(ctx, claimA, 1, lease); err != nil || len(first) != 1 {
			v.fail(clause, "OBX-17", "Claim(limit 1) = (%d, %v), want one record", len(first), err)
		} else if _, err := s.Store.MarkPublished(ctx, s.ID(first[0]), claimA); err != nil {
			v.fail(clause, "OBX-17", "MarkPublished = %v", err)
		} else if state, err := s.State(s.ID(first[0])); err != nil {
			v.fail(clause, "OBX-17", "State after MarkPublished = %v", err)
		} else {
			publishedAt := state.PublishedAt
			if removed, err := s.Purge(publishedAt); err != nil || removed != 0 {
				v.fail(clause, "OBX-17", "Purge(before = published_at) = (%d, %v), want 0 — the cutoff is exclusive", removed, err)
			}
			if removed, err := s.Purge(publishedAt + 1); err != nil || removed != 1 {
				v.fail(clause, "OBX-17", "Purge(after published_at) = (%d, %v), want exactly the published record", removed, err)
			}
			if s.Pending != nil {
				if pending, err := s.Pending(); err != nil || pending != 1 {
					v.fail(clause, "OBX-17", "Pending after the purge = (%d, %v), want 1 — the owed record stays", pending, err)
				}
			}
		}
	}

	return v
}
