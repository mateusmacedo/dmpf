package providerkit

import (
	"context"
	"sync"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// InboxSubject is what a realization gives the suite so it can drive Register
// inside a transaction and observe the committed status afterward. Within
// opens one transaction around fn and commits when fn returns nil; an error
// rolls back. A realization with a wait ceiling (Postgres lock_timeout) lets
// Concurrent run the two-insert race; one that serializes callers leaves it nil
// and the clause is reported as skipped.
type InboxSubject struct {
	Within     func(consumer string, fn func(ctx context.Context, inbox dmpfports.Inbox) error) error
	ReadStatus func(consumer string, id dmpfports.MessageID) (dmpfports.Status, bool)
	Concurrent bool
}

var errInboxRollback = errorString("providerkit: inbox rollback")

type errorString string

func (e errorString) Error() string { return string(e) }

func receipt(consumer string, id dmpfports.MessageID, hash string) dmpfports.Receipt {
	return dmpfports.Receipt{Consumer: consumer, MessageID: id, MessageType: "example", PayloadHash: hash, ReceivedAt: 100}
}

// match drives r.Match, completing the first branch as completeAs when non-nil,
// and reports which of the four branches ran.
func match(r dmpfports.Reception, completeAs *dmpfports.Status) (branch string, err error) {
	err = r.Match(
		func(p dmpfports.Pending) error {
			branch = "first"
			if completeAs == nil {
				return nil
			}
			return p.Complete(context.Background(), dmpfports.Completion{Status: *completeAs, At: 200})
		},
		func() error { branch = "processed"; return nil },
		func() error { branch = "rejected"; return nil },
		func() error { branch = "collision"; return nil },
	)
	return branch, err
}

func statusPtr(s dmpfports.Status) *dmpfports.Status { return &s }

// Inbox exercises the reception properties of FND-04 §6.2/§6.4 every
// realization must honour; newSubject must return a fresh, empty inbox.
func Inbox(newSubject func() InboxSubject) Verdict {
	var v Verdict

	registerAndCommit := func(clause string, s InboxSubject, consumer string, id dmpfports.MessageID, hash string, status dmpfports.Status) bool {
		err := s.Within(consumer, func(ctx context.Context, inbox dmpfports.Inbox) error {
			reception, err := inbox.Register(ctx, receipt(consumer, id, hash))
			if err != nil {
				return err
			}
			branch, err := match(reception, statusPtr(status))
			if err != nil {
				return err
			}
			if branch != "first" {
				return errorString("branch " + branch + ", want first for the first registration")
			}
			return nil
		})
		if err != nil {
			v.fail(clause, "INB-02", "first registration: %v", err)
			return false
		}
		return true
	}

	redeliver := func(s InboxSubject, consumer string, id dmpfports.MessageID, hash string) (string, error) {
		var branch string
		err := s.Within(consumer, func(ctx context.Context, inbox dmpfports.Inbox) error {
			reception, err := inbox.Register(ctx, receipt(consumer, id, hash))
			if err != nil {
				return err
			}
			branch, err = match(reception, nil)
			return err
		})
		return branch, err
	}

	{
		const clause = "first reception is R1"
		s := newSubject()
		if registerAndCommit(clause, s, "orders", "m-1", "h1", dmpfports.StatusProcessed) {
			if status, ok := s.ReadStatus("orders", "m-1"); !ok || status != dmpfports.StatusProcessed {
				v.fail(clause, "INB-02", "ReadStatus() = (%v, %v), want (processed, true)", status, ok)
			}
		}
	}

	for _, c := range []struct {
		clause string
		rule   string
		status dmpfports.Status
		want   string
	}{
		{"redelivery with the same hash after a processed commit is R2", "INB-06", dmpfports.StatusProcessed, "processed"},
		{"redelivery with the same hash after a rejected commit is R3", "INB-12", dmpfports.StatusRejected, "rejected"},
	} {
		s := newSubject()
		if !registerAndCommit(c.clause, s, "orders", "m-1", "h1", c.status) {
			continue
		}
		branch, err := redeliver(s, "orders", "m-1", "h1")
		if err != nil {
			v.fail(c.clause, c.rule, "Register() on redelivery = %v, want nil", err)
		} else if branch != c.want {
			v.fail(c.clause, c.rule, "branch = %q, want %q", branch, c.want)
		}
	}

	{
		const clause = "a divergent hash on a present key is R4"
		s := newSubject()
		if registerAndCommit(clause, s, "orders", "m-1", "h1", dmpfports.StatusProcessed) {
			branch, err := redeliver(s, "orders", "m-1", "h2")
			if err != nil {
				v.fail(clause, "INB-08", "Register() = %v, want nil", err)
			} else if branch != "collision" {
				v.fail(clause, "INB-08", "branch = %q, want collision", branch)
			}
		}
	}

	{
		const clause = "first reception again after a rollback"
		s := newSubject()
		err := s.Within("orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
			reception, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
			if err != nil {
				return err
			}
			if _, err := match(reception, statusPtr(dmpfports.StatusProcessed)); err != nil {
				return err
			}
			return errInboxRollback
		})
		if err != errInboxRollback { //nolint:errorlint // the sentinel is returned as is
			v.fail(clause, "INB-02", "Within() = %v, want the rollback sentinel", err)
		} else if _, ok := s.ReadStatus("orders", "m-1"); ok {
			v.fail(clause, "INB-02", "a rolled-back first reception must leave no committed row")
		} else {
			var branch string
			err := s.Within("orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
				reception, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
				if err != nil {
					return err
				}
				branch, err = match(reception, statusPtr(dmpfports.StatusProcessed))
				return err
			})
			if err != nil {
				v.fail(clause, "INB-02", "Register() after rollback = %v, want nil", err)
			} else if branch != "first" {
				v.fail(clause, "INB-02", "branch = %q, want first — nothing was ever applied", branch)
			}
		}
	}

	{
		const clause = "distinct consumers with the same MessageID do not dedupe"
		s := newSubject()
		if registerAndCommit(clause, s, "orders", "m-1", "h1", dmpfports.StatusProcessed) {
			if !registerAndCommit(clause, s, "billing", "m-1", "h1", dmpfports.StatusProcessed) {
				v.fail(clause, "INB-01", "a different consumer owns a disjoint key space")
			}
		}
	}

	{
		const clause = "a receipt naming another consumer is refused"
		s := newSubject()
		err := s.Within("orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
			_, err := inbox.Register(ctx, receipt("billing", "m-1", "h1"))
			if err == nil {
				return errorString("Register() = nil, want an error — the bound consumer owns the key")
			}
			return errInboxRollback
		})
		if err != errInboxRollback { //nolint:errorlint // the sentinel is returned as is
			v.fail(clause, "INB-01", "%v", err)
		}
	}

	{
		const clause = "registering a present key leaves the transaction usable"
		s := newSubject()
		if registerAndCommit(clause, s, "orders", "m-1", "h1", dmpfports.StatusProcessed) {
			err := s.Within("orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
				reception, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
				if err != nil {
					return errorString("a present key is a result, not a constraint error (INB-04): " + err.Error())
				}
				if _, err := match(reception, nil); err != nil {
					return err
				}
				reception, err = inbox.Register(ctx, receipt("orders", "m-2", "h2"))
				if err != nil {
					return errorString("a subsequent Register() in the same transaction: " + err.Error())
				}
				branch, err := match(reception, statusPtr(dmpfports.StatusProcessed))
				if err != nil {
					return err
				}
				if branch != "first" {
					return errorString("branch " + branch + ", want first for m-2")
				}
				return nil
			})
			if err != nil {
				v.fail(clause, "INB-04", "%v", err)
			} else if _, ok := s.ReadStatus("orders", "m-2"); !ok {
				v.fail(clause, "INB-04", "the second key must have committed alongside the first")
			}
		}
	}

	{
		const clause = "two concurrent first receptions resolve in exactly one winner"
		s := newSubject()
		if !s.Concurrent {
			v.skip(clause)
		} else {
			winners, err := race(s, "orders", "m-race", "h1")
			if err != nil {
				v.fail(clause, "INB-06", "%v", err)
			} else if winners != 1 {
				v.fail(clause, "INB-06", "%d transactions saw the first branch, want exactly 1", winners)
			} else if status, ok := s.ReadStatus("orders", "m-race"); !ok || status != dmpfports.StatusProcessed {
				v.fail(clause, "INB-06", "ReadStatus() after the race = (%v, %v), want (processed, true)", status, ok)
			}
		}
	}

	return v
}

// race opens two transactions that register the same key; each holds its
// registration until both have tried, so the second one is blocked (or waits
// out its ceiling) on the first's uncommitted row. Exactly one must see the
// first branch; the other sees processed or is refused by the ceiling and
// retries after the winner commits.
func race(s InboxSubject, consumer string, id dmpfports.MessageID, hash string) (winners int, err error) {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		errs     []error
		barrier  = make(chan struct{})
		started  sync.WaitGroup
		attempts = 2
	)
	started.Add(attempts)
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attempt := func() error {
				return s.Within(consumer, func(ctx context.Context, inbox dmpfports.Inbox) error {
					reception, err := inbox.Register(ctx, receipt(consumer, id, hash))
					if err != nil {
						return err
					}
					branch, err := match(reception, statusPtr(dmpfports.StatusProcessed))
					if err != nil {
						return err
					}
					if branch == "first" {
						mu.Lock()
						winners++
						mu.Unlock()
					}
					return nil
				})
			}
			started.Done()
			<-barrier
			if err := attempt(); err != nil {
				// A ceiling that expired is the realization refusing to wait;
				// after the winner commits the loser must resolve normally.
				if err2 := attempt(); err2 != nil {
					mu.Lock()
					errs = append(errs, err2)
					mu.Unlock()
				}
			}
		}()
	}
	started.Wait()
	close(barrier)
	wg.Wait()
	if len(errs) > 0 {
		return winners, errs[0]
	}
	return winners, nil
}
