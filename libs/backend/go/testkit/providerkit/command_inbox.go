package providerkit

import (
	"context"
	"errors"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// CommandInboxSubject drives the inbox of a context's commands: Within opens one
// transaction bound to it and commits when fn returns nil. A realization that
// serializes transactions leaves Concurrent false, and the in-flight clause is skipped.
type CommandInboxSubject struct {
	Within     func(ctx context.Context, fn func(ctx context.Context, inbox ports.Inbox) error) error
	Concurrent bool
}

const kitCommands = "kit.commands"

var errCommandRollback = errorString("providerkit: command rollback")

func commandReceipt(at ports.Instant, hash string) ports.Receipt {
	return ports.Receipt{
		Consumer:    kitCommands,
		MessageID:   "dmpf-kit-key",
		MessageType: "kit.Command",
		PayloadHash: hash,
		ReceivedAt:  at,
		WaitUntil:   at + ports.Instant(time.Second),
		ExpiresAt:   at + ports.Instant(time.Hour),
	}
}

type commandRun struct {
	branch string
	stored []byte
}

func (s CommandInboxSubject) run(ctx context.Context, r ports.Receipt, status ports.Status, outcome string) (commandRun, error) {
	var got commandRun
	err := s.Within(ctx, func(ctx context.Context, inbox ports.Inbox) error {
		reception, err := inbox.Register(ctx, r)
		if err != nil {
			return err
		}
		return reception.Match(
			func(p ports.Pending) error {
				got.branch = "first"
				return p.Complete(ctx, ports.Completion{Status: status, At: r.ReceivedAt, Outcome: []byte(outcome)})
			},
			func() error { got.branch, got.stored = "processed", reception.Stored(); return nil },
			func() error { got.branch, got.stored = "rejected", reception.Stored(); return nil },
			func() error { got.branch = "collision"; return nil },
		)
	})
	return got, err
}

// CommandInbox exercises the inbox of a context's commands (FND-04 IDM-*): the
// replay of both outcomes, the scope by tenant and the expiry. newSubject must
// return a fresh, empty inbox.
func CommandInbox(newSubject func() CommandInboxSubject) Verdict {
	var v Verdict

	acme, err := scopedTo("acme")
	if err != nil {
		v.fail("setup", "IDM-03", "scopedTo(acme): %v", err)
		return v
	}
	globex, err := scopedTo("globex")
	if err != nil {
		v.fail("setup", "IDM-03", "scopedTo(globex): %v", err)
		return v
	}

	for _, c := range []struct {
		clause string
		rule   string
		status ports.Status
		want   string
	}{
		{"a repeated command replays the accepted outcome", "IDM-08", ports.StatusProcessed, "processed"},
		{"a repeated command replays the rejection", "IDM-06", ports.StatusRejected, "rejected"},
	} {
		s := newSubject()
		if first, err := s.run(acme, commandReceipt(1_000, "h1"), c.status, "outcome-1"); err != nil || first.branch != "first" {
			v.fail(c.clause, c.rule, "first command = (%s, %v), want (first, nil)", first.branch, err)
			continue
		}
		again, err := s.run(acme, commandReceipt(2_000, "h1"), c.status, "outcome-2")
		if err != nil || again.branch != c.want || string(again.stored) != "outcome-1" {
			v.fail(c.clause, c.rule, "repeat = (%s, %q, %v), want (%s, outcome-1, nil)", again.branch, again.stored, err, c.want)
		}
	}

	{
		const clause = "a rolled-back command leaves no entry"
		s := newSubject()
		err := s.Within(acme, func(ctx context.Context, inbox ports.Inbox) error {
			reception, err := inbox.Register(ctx, commandReceipt(1_000, "h1"))
			if err != nil {
				return err
			}
			if err := reception.Match(
				func(p ports.Pending) error {
					return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: 1_000, Outcome: []byte("outcome-1")})
				},
				func() error { return nil }, func() error { return nil }, func() error { return nil },
			); err != nil {
				return err
			}
			return errCommandRollback
		})
		if !errors.Is(err, errCommandRollback) {
			v.fail(clause, "IDM-05", "Within() = %v, want the callback error", err)
		} else if again, err := s.run(acme, commandReceipt(2_000, "h1"), ports.StatusProcessed, "outcome-2"); err != nil || again.branch != "first" {
			v.fail(clause, "IDM-05", "command after rollback = (%s, %v), want (first, nil)", again.branch, err)
		}
	}

	{
		const clause = "the same key with another payload collides"
		s := newSubject()
		if _, err := s.run(acme, commandReceipt(1_000, "h1"), ports.StatusProcessed, "outcome-1"); err != nil {
			v.fail(clause, "IDM-04", "first command: %v", err)
		} else if again, err := s.run(acme, commandReceipt(2_000, "h2"), ports.StatusProcessed, "outcome-2"); err != nil || again.branch != "collision" {
			v.fail(clause, "IDM-04", "another payload = (%s, %v), want (collision, nil)", again.branch, err)
		}
	}

	{
		const clause = "the key is scoped to the tenant"
		s := newSubject()
		if _, err := s.run(acme, commandReceipt(1_000, "h1"), ports.StatusProcessed, "acme-outcome"); err != nil {
			v.fail(clause, "IDM-03", "acme command: %v", err)
		} else if other, err := s.run(globex, commandReceipt(2_000, "h1"), ports.StatusProcessed, "globex-outcome"); err != nil || other.branch != "first" {
			v.fail(clause, "IDM-03", "globex = (%s, %q, %v), want (first, nil): another tenant's outcome must never replay", other.branch, other.stored, err)
		}
	}

	{
		const clause = "an expired entry is replaced as if absent"
		s := newSubject()
		stale := commandReceipt(1_000, "h1")
		if _, err := s.run(acme, stale, ports.StatusProcessed, "stale"); err != nil {
			v.fail(clause, "IDM-09", "first command: %v", err)
		} else if fresh, err := s.run(acme, commandReceipt(stale.ExpiresAt, "h2"), ports.StatusProcessed, "fresh"); err != nil || fresh.branch != "first" {
			v.fail(clause, "IDM-09", "command after expiry = (%s, %v), want (first, nil)", fresh.branch, err)
		} else if replay, err := s.run(acme, commandReceipt(stale.ExpiresAt+1, "h2"), ports.StatusProcessed, "unused"); err != nil || string(replay.stored) != "fresh" {
			v.fail(clause, "IDM-09", "replay of the replacement = (%s, %q, %v), want (processed, fresh, nil)", replay.branch, replay.stored, err)
		}
	}

	{
		const clause = "a command without a tenant is refused"
		s := newSubject()
		ctx, err := tenantless()
		if err != nil {
			v.fail(clause, "IDN-15", "tenantless(): %v", err)
		} else if _, err := s.run(ctx, commandReceipt(1_000, "h1"), ports.StatusProcessed, "outcome"); err == nil {
			v.fail(clause, "IDN-15", "command without tenant = nil, want a refusal")
		}
	}

	{
		const clause = "a command without an expiry is refused"
		s := newSubject()
		receipt := commandReceipt(1_000, "h1")
		receipt.ExpiresAt = 0
		if _, err := s.run(acme, receipt, ports.StatusProcessed, "outcome"); err == nil {
			v.fail(clause, "IDM-09", "command without expiry = nil, want a refusal: its entry would never be purged")
		}
	}

	commandInFlight(newSubject, acme, &v)
	return v
}

func commandInFlight(newSubject func() CommandInboxSubject, ctx context.Context, v *Verdict) {
	const clause = "a concurrent command past its ceiling is in flight"
	s := newSubject()
	if !s.Concurrent {
		v.skip(clause)
		return
	}

	registered := make(chan struct{})
	release := make(chan struct{})
	holder := make(chan error, 1)
	go func() {
		holder <- s.Within(ctx, func(ctx context.Context, inbox ports.Inbox) error {
			reception, err := inbox.Register(ctx, commandReceipt(1_000, "h1"))
			close(registered)
			if err != nil {
				return err
			}
			<-release
			return reception.Match(
				func(p ports.Pending) error {
					return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: 1_000, Outcome: []byte("holder")})
				},
				func() error { return nil }, func() error { return nil }, func() error { return nil },
			)
		})
	}()
	<-registered

	short := commandReceipt(2_000, "h1")
	short.WaitUntil = short.ReceivedAt + ports.Instant(100*time.Millisecond)
	_, err := s.run(ctx, short, ports.StatusProcessed, "unused")
	close(release)
	if holdErr := <-holder; holdErr != nil {
		v.fail(clause, "IDM-07", "holder: %v", holdErr)
		return
	}
	if !errors.Is(err, ports.ErrRegisterTimeout) {
		v.fail(clause, "IDM-07", "command during the holder = %v, want ErrRegisterTimeout", err)
	}
}
