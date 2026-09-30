//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const probeCommands = "probes.commands"

func bindCommands(tx *postgres.Tx) ports.Inbox { return tx.CommandInbox(probeCommands, 2*time.Second) }

func probeCommand(at ports.Instant) ports.Receipt {
	return ports.Receipt{
		Consumer:    probeCommands,
		MessageID:   "k-1",
		MessageType: "probes.Write",
		PayloadHash: "h1",
		ReceivedAt:  at,
		WaitUntil:   at + ports.Instant(time.Second),
		ExpiresAt:   at + ports.Instant(24*time.Hour),
	}
}

func command(ctx context.Context, uow ports.UnitOfWork[ports.Inbox], r ports.Receipt, outcome string) (string, []byte, error) {
	var (
		branch string
		stored []byte
	)
	err := uow.Within(ctx, func(ctx context.Context, inbox ports.Inbox) error {
		reception, err := inbox.Register(ctx, r)
		if err != nil {
			return err
		}
		return reception.Match(
			func(p ports.Pending) error {
				branch = "first"
				return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: r.ReceivedAt, Outcome: []byte(outcome)})
			},
			func() error { branch, stored = "processed", reception.Stored(); return nil },
			func() error { branch, stored = "rejected", reception.Stored(); return nil },
			func() error { branch = "collision"; return nil },
		)
	})
	return branch, stored, err
}

type commandResult struct {
	branch string
	stored []byte
	err    error
}

func commandAsync(ctx context.Context, uow ports.UnitOfWork[ports.Inbox], r ports.Receipt) <-chan commandResult {
	done := make(chan commandResult, 1)
	go func() {
		branch, stored, err := command(ctx, uow, r, "unused")
		done <- commandResult{branch, stored, err}
	}()
	return done
}

type heldCommand struct {
	release chan struct{}
	done    chan error
}

func holdCommand(ctx context.Context, uow ports.UnitOfWork[ports.Inbox], r ports.Receipt, outcome string) heldCommand {
	h := heldCommand{release: make(chan struct{}), done: make(chan error, 1)}
	registered := make(chan struct{})
	go func() {
		h.done <- uow.Within(ctx, func(ctx context.Context, inbox ports.Inbox) error {
			reception, err := inbox.Register(ctx, r)
			close(registered)
			if err != nil {
				return err
			}
			<-h.release
			return reception.Match(
				func(p ports.Pending) error {
					return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: r.ReceivedAt, Outcome: []byte(outcome)})
				},
				func() error { return nil }, func() error { return nil }, func() error { return nil },
			)
		})
	}()
	<-registered
	return h
}

func awaitLockWaiter(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		if err := pool.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock')`,
		).Scan(&waiting); err != nil {
			t.Fatalf("pg_stat_activity = %v", err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no session waits on a lock after 5s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestACommandWaitingOnItsKeyReplaysOnceTheHolderCommits(t *testing.T) {
	pool := openPool(t)
	uow := postgres.NewUnitOfWork(pool, bindCommands)
	ctx := scopedTo(t, "acme")

	holder := holdCommand(ctx, uow, probeCommand(1_000), "holder")
	waiter := commandAsync(ctx, uow, probeCommand(2_000))
	awaitLockWaiter(t, pool)
	close(holder.release)

	if err := <-holder.done; err != nil {
		t.Fatalf("holder = %v, want nil", err)
	}
	got := <-waiter
	if got.err != nil || got.branch != "processed" || string(got.stored) != "holder" {
		t.Fatalf("waiter = (%s, %q, %v), want (processed, holder, nil): it waited within the ceiling and replays", got.branch, got.stored, got.err)
	}
}

func TestTwoCommandsReplacingAnExpiredEntryExecuteOnce(t *testing.T) {
	pool := openPool(t)
	uow := postgres.NewUnitOfWork(pool, bindCommands)
	ctx := scopedTo(t, "acme")

	expired := probeCommand(1_000)
	expired.ExpiresAt = 1_500
	if _, _, err := command(ctx, uow, expired, "expired"); err != nil {
		t.Fatalf("seeding the expired entry = %v, want nil", err)
	}

	holder := holdCommand(ctx, uow, probeCommand(2_000), "holder")
	waiter := commandAsync(ctx, uow, probeCommand(3_000))
	awaitLockWaiter(t, pool)
	close(holder.release)

	if err := <-holder.done; err != nil {
		t.Fatalf("holder = %v, want nil", err)
	}
	got := <-waiter
	if got.err != nil || got.branch != "processed" || string(got.stored) != "holder" {
		t.Fatalf("waiter = (%s, %q, %v), want (processed, holder, nil): the entry the holder put in place is no longer expired", got.branch, got.stored, got.err)
	}
}

func TestACommandGivesUpOnItsKeyBeforeTheRequestDeadline(t *testing.T) {
	pool := openPool(t)
	uow := postgres.NewUnitOfWork(pool, bindCommands)
	ctx := scopedTo(t, "acme")

	holder := holdCommand(ctx, uow, probeCommand(1_000), "holder")
	defer func() { close(holder.release); <-holder.done }()

	bounded, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if _, _, err := command(bounded, uow, probeCommand(2_000), "unused"); !errors.Is(err, ports.ErrRegisterTimeout) {
		t.Fatalf("command under a 500ms deadline = %v, want ErrRegisterTimeout: the 1s wait of the key shrinks to the deadline of the request", err)
	}
}

func TestACommandCeilingAtItsReceptionNeverWaitsForever(t *testing.T) {
	pool := openPool(t)
	uow := postgres.NewUnitOfWork(pool, bindCommands)
	ctx := scopedTo(t, "acme")

	registered := make(chan struct{})
	release := make(chan struct{})
	holder := make(chan error, 1)
	go func() {
		holder <- uow.Within(ctx, func(ctx context.Context, inbox ports.Inbox) error {
			_, err := inbox.Register(ctx, probeCommand(1_000))
			close(registered)
			<-release
			if err != nil {
				return err
			}
			return errors.New("postgres_test: holder rolls back")
		})
	}()
	<-registered
	defer func() { close(release); <-holder }()

	r := probeCommand(2_000)
	r.WaitUntil = r.ReceivedAt
	started := time.Now()
	_, _, err := command(ctx, uow, r, "unused")
	if !errors.Is(err, ports.ErrRegisterTimeout) {
		t.Fatalf("command with no wait left = %v, want ErrRegisterTimeout", err)
	}
	if waited := time.Since(started); waited > time.Second {
		t.Fatalf("waited %v with no ceiling left; lock_timeout = 0 would have waited forever", waited)
	}
}

type countingTracer struct{ statements atomic.Int64 }

func (c *countingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.statements.Add(1)
	return ctx
}

func (*countingTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestACommandCostsAtMostFiveStatements(t *testing.T) {
	tracer := &countingTracer{}
	pool := openPoolWith(t, func(cfg *pgxpool.Config) { cfg.ConnConfig.Tracer = tracer })
	uow := postgres.NewUnitOfWork(pool, bindCommands)
	ctx := scopedTo(t, "acme")

	measure := func(r ports.Receipt) int64 {
		var spent int64
		err := uow.Within(ctx, func(ctx context.Context, inbox ports.Inbox) error {
			before := tracer.statements.Load()
			reception, err := inbox.Register(ctx, r)
			if err != nil {
				return err
			}
			if err := reception.Match(
				func(p ports.Pending) error {
					return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: r.ReceivedAt, Outcome: []byte("outcome")})
				},
				func() error { return nil }, func() error { return nil }, func() error { return nil },
			); err != nil {
				return err
			}
			spent = tracer.statements.Load() - before
			return nil
		})
		if err != nil {
			t.Fatalf("Within() = %v, want nil", err)
		}
		return spent
	}

	if spent := measure(probeCommand(1_000)); spent > 5 {
		t.Errorf("first execution spent %d statements, want at most 5", spent)
	}
	if spent := measure(probeCommand(2_000)); spent > 5 {
		t.Errorf("replay spent %d statements, want at most 5", spent)
	}
}
