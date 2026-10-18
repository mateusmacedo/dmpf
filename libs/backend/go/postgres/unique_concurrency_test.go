//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const uniqueRounds = 50

var errRedelivered = errors.New("postgres_test: the command was already registered")

type decision struct{ saved, err error }

func uniqueReceipt(id string) ports.Receipt {
	r := probeCommand(100)
	r.MessageID = ports.MessageID(id)
	return r
}

// decide is what RunIdempotent does around one command: the inbox registers it,
// the Save runs inside the first reception, and a declared refusal completes the
// inbox as rejected on the same transaction instead of failing it.
func decide(ctx context.Context, tx *postgres.Tx, messageID, id string, state uniqueProbe) decision {
	var saved error
	reception, err := tx.CommandInbox(probeCommands, 2*time.Second).Register(ctx, uniqueReceipt(messageID))
	if err != nil {
		return decision{err: err}
	}
	err = reception.Match(
		func(p ports.Pending) error {
			saved = uniqueTable.Repository(tx).Save(ctx, id, state, 0)
			if violation, ok := errors.AsType[ports.UniqueViolation](saved); ok && violation.Rejection != nil {
				return p.Complete(ctx, ports.Completion{Status: ports.StatusRejected, At: 200, LastError: string(violation.Rejection.Code())})
			}
			if saved != nil {
				return saved
			}
			return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: 200})
		},
		func() error { return errRedelivered },
		func() error { return errRedelivered },
		func() error { return errRedelivered },
	)
	return decision{saved: saved, err: err}
}

func decideAsync(ctx context.Context, tx *postgres.Tx, messageID, id string, state uniqueProbe) <-chan decision {
	done := make(chan decision, 1)
	go func() {
		d := decide(ctx, tx, messageID, id, state)
		if d.err == nil {
			d.err = postgres.ConnOf(tx).Commit(ctx)
		} else {
			_ = postgres.ConnOf(tx).Rollback(ctx)
		}
		done <- d
	}()
	return done
}

// awaitLockWait holds A's commit until B is provably blocked on it; without it a
// round could pass with B arriving after the commit, never racing at all.
func awaitLockWait(t *testing.T, admin *pgxpool.Pool, pid uint32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		err := admin.QueryRow(context.Background(), `
			SELECT EXISTS (SELECT 1 FROM pg_locks WHERE pid = $1 AND NOT granted)
			   AND EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = $1 AND wait_event_type = 'Lock')`, int64(pid)).Scan(&waiting)
		if err != nil {
			t.Fatalf("pg_locks of pid %d = %v", pid, err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("B (pid %d) never waited on A's lock; the round would not exercise the race", pid)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query).Scan(&n); err != nil {
		t.Fatalf("%s = %v", query, err)
	}
	return n
}

func TestAConcurrentDuplicateIsRefusedAndTheRefusalCommits(t *testing.T) {
	pool := openConcurrencyPool(t)
	admin := openAdminPool(t)
	ctx := scopedTo(t, "acme")

	for round := range uniqueRounds {
		n := fmt.Sprint(round)
		name := "dup-" + n

		a := begin(t, pool)
		if d := decide(ctx, a, "a-"+n, "A-"+n, uniqueProbe{Name: name, Code: "a-" + n, Label: "a-" + n, RefA: "a-" + n, RefB: "a-" + n}); d.saved != nil || d.err != nil {
			t.Fatalf("round %d: A = %+v, want its write pending", round, d)
		}
		b := begin(t, pool)
		pid := postgres.ConnOf(b).Conn().PgConn().PID()
		done := decideAsync(ctx, b, "b-"+n, "B-"+n, uniqueProbe{Name: name, Code: "b-" + n, Label: "b-" + n, RefA: "b-" + n, RefB: "b-" + n})

		awaitLockWait(t, admin, pid)
		if err := postgres.ConnOf(a).Commit(ctx); err != nil {
			t.Fatalf("round %d: A Commit() = %v", round, err)
		}

		d := <-done
		if d.err != nil {
			t.Fatalf("round %d: B = %v, want the refusal committed on the transaction the violation left usable", round, d.err)
		}
		requireViolation(t, d.saved, "unique_probes_tenant_id_name_key", nameTaken)
	}

	if got := countRows(t, admin, `SELECT count(*) FROM inbox WHERE message_id LIKE 'acme/b-%' AND status = 'rejected'`); got != uniqueRounds {
		t.Fatalf("rejected B commands = %d, want %d", got, uniqueRounds)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM inbox WHERE message_id LIKE 'acme/a-%' AND status = 'processed'`); got != uniqueRounds {
		t.Fatalf("processed A commands = %d, want %d", got, uniqueRounds)
	}
	if got := countRows(t, admin, `SELECT count(*) FROM unique_probes WHERE probe_id LIKE 'B-%'`); got != 0 {
		t.Fatalf("B rows = %d, want none: the savepoint undid every refused write", got)
	}
}

func TestAConcurrentCreateOfTheSameIdentityIsAVersionConflict(t *testing.T) {
	pool := openConcurrencyPool(t)
	admin := openAdminPool(t)
	ctx := scopedTo(t, "acme")

	for round := range uniqueRounds {
		n := fmt.Sprint(round)
		id, state := "S-"+n, uniqueProbe{Name: "same-" + n, Code: "same-" + n, Label: "same-" + n, RefA: "same-" + n, RefB: "same-" + n}

		a := begin(t, pool)
		if d := decide(ctx, a, "a-"+n, id, state); d.saved != nil || d.err != nil {
			t.Fatalf("round %d: A = %+v, want its write pending", round, d)
		}
		b := begin(t, pool)
		pid := postgres.ConnOf(b).Conn().PgConn().PID()
		done := decideAsync(ctx, b, "b-"+n, id, state)

		awaitLockWait(t, admin, pid)
		if err := postgres.ConnOf(a).Commit(ctx); err != nil {
			t.Fatalf("round %d: A Commit() = %v", round, err)
		}

		d := <-done
		if !errors.Is(d.saved, ports.ErrVersionConflict) || errors.Is(d.saved, ports.ErrUniqueViolation) {
			t.Fatalf("round %d: B Save() = %v, want ErrVersionConflict from ON CONFLICT: the same identity is a lost update, never a refusal", round, d.saved)
		}
	}

	if got := countRows(t, admin, `SELECT count(*) FROM inbox WHERE message_id LIKE 'acme/b-%'`); got != 0 {
		t.Fatalf("B inbox rows = %d, want none: a conflict rolls the whole command back", got)
	}
}
