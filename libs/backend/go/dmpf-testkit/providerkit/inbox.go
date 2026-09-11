package providerkit

import (
	"context"
	"errors"
	"sync"
	"time"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// InboxSubject is what a realization gives the suite so it can drive Register
// inside a transaction and observe the committed status afterward. Within
// opens one transaction under ctx around fn and commits when fn returns nil;
// an error rolls back, and so does a ctx that expires while fn is inside the
// realization. A realization with a wait ceiling (Postgres lock_timeout) lets
// Concurrent run the two-insert race; one that serializes callers leaves it nil
// and the clause is reported as skipped.
type InboxSubject struct {
	Within     func(ctx context.Context, consumer string, fn func(ctx context.Context, inbox dmpfports.Inbox) error) error
	ReadStatus func(consumer string, id dmpfports.MessageID) (dmpfports.Status, bool)
	Concurrent bool

	// ConsumerMismatch is the realization's own error for a receipt naming
	// another consumer; when set, the refusal must be reachable by errors.Is.
	ConsumerMismatch error

	// Rows counts the committed inbox rows, when the realization can tell; nil
	// leaves the count unchecked. It is how a redelivery proves it wrote nothing.
	Rows func() int
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
		err := s.Within(context.Background(), consumer, func(ctx context.Context, inbox dmpfports.Inbox) error {
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
		err := s.Within(context.Background(), consumer, func(ctx context.Context, inbox dmpfports.Inbox) error {
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
		switch {
		case err != nil:
			v.fail(c.clause, c.rule, "Register() on redelivery = %v, want nil", err)
		case branch != c.want:
			v.fail(c.clause, c.rule, "branch = %q, want %q", branch, c.want)
		case s.Rows != nil && s.Rows() != 1:
			v.fail(c.clause, c.rule, "%d inbox rows after the redelivery, want 1 — a redelivery writes nothing", s.Rows())
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
		err := s.Within(context.Background(), "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
			reception, err := inbox.Register(ctx, receipt("orders", "m-1", "h1"))
			if err != nil {
				return err
			}
			if _, err := match(reception, statusPtr(dmpfports.StatusProcessed)); err != nil {
				return err
			}
			return errInboxRollback
		})
		if !errors.Is(err, errInboxRollback) {
			v.fail(clause, "INB-02", "Within() = %v, want the rollback sentinel", err)
		} else if _, ok := s.ReadStatus("orders", "m-1"); ok {
			v.fail(clause, "INB-02", "a rolled-back first reception must leave no committed row")
		} else {
			var branch string
			err := s.Within(context.Background(), "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
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
		var refused error
		err := s.Within(context.Background(), "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
			_, refused = inbox.Register(ctx, receipt("billing", "m-1", "h1"))
			if refused == nil {
				return errorString("Register() = nil, want an error — the bound consumer owns the key")
			}
			return errInboxRollback
		})
		if !errors.Is(err, errInboxRollback) {
			v.fail(clause, "INB-01", "%v", err)
		} else if s.ConsumerMismatch != nil && !errors.Is(refused, s.ConsumerMismatch) {
			v.fail(clause, "INB-01", "Register() = %v, want the realization's consumer-mismatch error", refused)
		}
	}

	{
		const clause = "registering a present key leaves the transaction usable"
		s := newSubject()
		if registerAndCommit(clause, s, "orders", "m-1", "h1", dmpfports.StatusProcessed) {
			err := s.Within(context.Background(), "orders", func(ctx context.Context, inbox dmpfports.Inbox) error {
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

// race opens two transactions that register the same key and makes them
// overlap: each signals when its Register returned — with a first reception,
// a redelivery, or the realization's wait ceiling — and the one holding the
// first reception completes only after both returned, so the other was inside
// Register against an uncommitted row. Exactly one transaction may win; the
// other sees processed, or was refused by the ceiling and retries after the
// winner committed. A realization that blocks on the key without a ceiling
// never lets the other Register return, and the deadline names that.
func race(s InboxSubject, consumer string, id dmpfports.MessageID, hash string) (winners int, err error) {
	const attempts = 2
	// The deadline is also the transactions' ctx: when it expires, a Register
	// still blocked on the key is cancelled instead of outliving the verdict.
	ctx, cancel := context.WithTimeout(context.Background(), raceDeadline)
	defer cancel()
	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		errs       []error
		barrier    = make(chan struct{})
		started    sync.WaitGroup
		registered sync.WaitGroup
	)
	started.Add(attempts)
	registered.Add(attempts)
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signalled := false
			attempt := func() (branch string, err error) {
				err = s.Within(ctx, consumer, func(ctx context.Context, inbox dmpfports.Inbox) error {
					reception, err := inbox.Register(ctx, receipt(consumer, id, hash))
					if !signalled {
						signalled = true
						registered.Done()
					}
					if err != nil {
						return err
					}
					return reception.Match(
						func(p dmpfports.Pending) error {
							branch = "first"
							registered.Wait()
							return p.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusProcessed, At: 200})
						},
						func() error { branch = "processed"; return nil },
						func() error { branch = "rejected"; return nil },
						func() error { branch = "collision"; return nil },
					)
				})
				return branch, err
			}
			started.Done()
			<-barrier
			branch, err := attempt()
			if err != nil {
				// A ceiling that expired is the realization refusing to wait;
				// after the winner commits the loser must resolve normally.
				if branch, err = attempt(); err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
					return
				}
			}
			if branch == "first" {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	started.Wait()
	close(barrier)
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-ctx.Done():
		mu.Lock()
		defer mu.Unlock()
		return winners, errorString("providerkit: the two transactions did not finish within " + raceDeadline.String() + " — a realization without a wait ceiling cannot run the race")
	}
	if len(errs) > 0 {
		return winners, errs[0]
	}
	return winners, nil
}

// raceDeadline bounds the race so a candidate that blocks forever on the key
// fails the clause instead of hanging the suite.
const raceDeadline = 60 * time.Second
