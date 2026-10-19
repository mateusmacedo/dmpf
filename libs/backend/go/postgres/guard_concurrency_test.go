//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

type entry struct {
	ID, Owner, Status, Supersedes string
}

type head struct{ Stage string }

var seriesTable = postgres.Table[string, entry]{
	Name:     "guarded_series",
	IDColumn: "entry_id",
	Columns:  []string{"owner_id", "status", "supersedes_id"},
	Encode: func(e entry) ([]any, error) {
		var supersedes *string
		if e.Supersedes != "" {
			supersedes = &e.Supersedes
		}
		return []any{e.Owner, e.Status, supersedes}, nil
	},
	Decode: func(scan func(dest ...any) error) (entry, error) {
		var (
			e          entry
			supersedes *string
		)
		err := scan(&e.Owner, &e.Status, &supersedes)
		if supersedes != nil {
			e.Supersedes = *supersedes
		}
		return e, err
	},
	WithID: func(e entry, id string) entry {
		e.ID = id
		return e
	},
	Guard: "owner_id",
}

var headTable = postgres.Table[string, head]{
	Name:     "guarded_heads",
	IDColumn: "head_id",
	Columns:  []string{"stage"},
	Encode:   func(h head) ([]any, error) { return []any{h.Stage}, nil },
	Decode: func(scan func(dest ...any) error) (head, error) {
		var h head
		err := scan(&h.Stage)
		return h, err
	},
}

var errNotEnabled = errors.New("postgres_test: no current entry of the series is enabled")

func currentlyEnabled(series []entry) bool {
	superseded := map[string]bool{}
	for _, e := range series {
		if e.Supersedes != "" {
			superseded[e.Supersedes] = true
		}
	}
	for _, e := range series {
		if !superseded[e.ID] && e.Status == "enabled" {
			return true
		}
	}
	return false
}

func checkEnabled(ctx context.Context, tx *postgres.Tx, owner string) error {
	series, err := seriesTable.Relation("owner_id").QueryWithin(ctx, tx, owner)
	if err != nil {
		return err
	}
	if !currentlyEnabled(series) {
		return errNotEnabled
	}
	return nil
}

func activate(ctx context.Context, tx *postgres.Tx, headID string) error {
	repo := headTable.Repository(tx)
	h, version, err := repo.Load(ctx, headID)
	if err != nil {
		return err
	}
	h.Stage = "active"
	return repo.Save(ctx, headID, h, version)
}

type seriesWrite struct {
	name  string
	apply func(ctx context.Context, tx *postgres.Tx) error
}

var seriesWrites = []seriesWrite{
	{"an update of the read entry", func(ctx context.Context, tx *postgres.Tx) error {
		repo := seriesTable.Repository(tx)
		e, version, err := repo.Load(ctx, "E-1")
		if err != nil {
			return err
		}
		e.Status = "suspended"
		return repo.Save(ctx, "E-1", e, version)
	}},
	{"an insert superseding the read entry", func(ctx context.Context, tx *postgres.Tx) error {
		return seriesTable.Repository(tx).Save(ctx, "E-2", entry{Owner: "O-1", Status: "declared", Supersedes: "E-1"}, 0)
	}},
}

func seedSeries(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	tx := begin(t, pool)
	if err := seriesTable.Repository(tx).Save(ctx, "E-1", entry{Owner: "O-1", Status: "enabled"}, 0); err != nil {
		t.Fatalf("seed E-1 = %v", err)
	}
	if err := headTable.Repository(tx).Save(ctx, "H-1", head{Stage: "homologating"}, 0); err != nil {
		t.Fatalf("seed H-1 = %v", err)
	}
	if err := postgres.ConnOf(tx).Commit(ctx); err != nil {
		t.Fatalf("seed Commit() = %v", err)
	}
}

func commitAsync(ctx context.Context, tx *postgres.Tx, run func() error) <-chan error {
	done := make(chan error, 1)
	go func() {
		err := run()
		if err == nil {
			err = postgres.ConnOf(tx).Commit(ctx)
		} else {
			_ = postgres.ConnOf(tx).Rollback(ctx)
		}
		done <- err
	}()
	return done
}

func stageOf(t *testing.T, admin *pgxpool.Pool, headID string) string {
	t.Helper()
	var stage string
	if err := admin.QueryRow(context.Background(), "SELECT stage FROM guarded_heads WHERE head_id = $1", headID).Scan(&stage); err != nil {
		t.Fatalf("stage of %s = %v", headID, err)
	}
	return stage
}

func TestTheGuardMakesTheSeriesWriteWaitForThePreconditionRead(t *testing.T) {
	for _, write := range seriesWrites {
		t.Run(write.name, func(t *testing.T) {
			pool := openConcurrencyPool(t)
			admin := openAdminPool(t)
			ctx := scopedTo(t, "acme")
			seedSeries(t, ctx, pool)

			a := begin(t, pool)
			if err := checkEnabled(ctx, a, "O-1"); err != nil {
				t.Fatalf("A: precondition = %v, want the seeded series enabled", err)
			}
			b := begin(t, pool)
			pid := postgres.ConnOf(b).Conn().PgConn().PID()
			done := commitAsync(ctx, b, func() error { return write.apply(ctx, b) })

			awaitLockWait(t, admin, pid)
			if err := activate(ctx, a, "H-1"); err != nil {
				t.Fatalf("A: activate = %v", err)
			}
			if err := postgres.ConnOf(a).Commit(ctx); err != nil {
				t.Fatalf("A: Commit() = %v", err)
			}
			if err := <-done; err != nil {
				t.Fatalf("B = %v, want it committed after A", err)
			}
			if got := stageOf(t, admin, "H-1"); got != "active" {
				t.Fatalf("H-1 stage = %q, want active: A decided first and committed first", got)
			}
			if got := countRows(t, admin, "SELECT count(*) FROM guarded_series WHERE status <> 'enabled'"); got != 1 {
				t.Fatalf("series rows B changed = %d, want 1: B committed after A", got)
			}
		})
	}
}

func TestTheGuardMakesThePreconditionReadWaitForTheSeriesWrite(t *testing.T) {
	for _, write := range seriesWrites {
		t.Run(write.name, func(t *testing.T) {
			pool := openConcurrencyPool(t)
			admin := openAdminPool(t)
			ctx := scopedTo(t, "acme")
			seedSeries(t, ctx, pool)

			b := begin(t, pool)
			if err := write.apply(ctx, b); err != nil {
				t.Fatalf("B = %v, want its write pending", err)
			}
			a := begin(t, pool)
			pid := postgres.ConnOf(a).Conn().PgConn().PID()
			done := commitAsync(ctx, a, func() error {
				if err := checkEnabled(ctx, a, "O-1"); err != nil {
					return err
				}
				return activate(ctx, a, "H-1")
			})

			awaitLockWait(t, admin, pid)
			if err := postgres.ConnOf(b).Commit(ctx); err != nil {
				t.Fatalf("B: Commit() = %v", err)
			}
			if err := <-done; !errors.Is(err, errNotEnabled) {
				t.Fatalf("A = %v, want %v: it read the series B committed", err, errNotEnabled)
			}
			if got := stageOf(t, admin, "H-1"); got != "homologating" {
				t.Fatalf("H-1 stage = %q, want homologating: no decision over the overwritten series", got)
			}
		})
	}
}

const guardRounds = 50

type preconditionRead func(ctx context.Context, tx *postgres.Tx, owner string) ([]entry, error)

type round struct {
	readEnabled             bool
	aCommitting, bCommitted time.Time
	errA, errB              error
}

// B committing between A's read and A's commit is the race itself: the final rows
// match the serial order A then B, so only the timing tells the two apart.
func (r round) stale() bool {
	return r.errA == nil && r.errB == nil && r.readEnabled && r.bCommitted.Before(r.aCommitting)
}

func raceOnce(t *testing.T, ctx context.Context, pool *pgxpool.Pool, read preconditionRead, write seriesWrite) round {
	t.Helper()
	a, b := begin(t, pool), begin(t, pool)
	start := make(chan struct{})
	var (
		r  round
		wg sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		series, err := read(ctx, a, "O-1")
		if err == nil && currentlyEnabled(series) {
			r.readEnabled = true
			time.Sleep(2 * time.Millisecond)
			err = activate(ctx, a, "H-1")
		}
		if err != nil {
			r.errA = err
			return
		}
		r.aCommitting = time.Now()
		r.errA = postgres.ConnOf(a).Commit(ctx)
	}()
	go func() {
		defer wg.Done()
		<-start
		if r.errB = write.apply(ctx, b); r.errB != nil {
			return
		}
		r.errB = postgres.ConnOf(b).Commit(ctx)
		r.bCommitted = time.Now()
	}()
	close(start)
	wg.Wait()
	return r
}

func staleRounds(t *testing.T, read func(pool *pgxpool.Pool) preconditionRead, write seriesWrite) int {
	t.Helper()
	pool := openConcurrencyPool(t)
	admin := openAdminPool(t)
	ctx := scopedTo(t, "acme")
	stale := 0
	for n := range guardRounds {
		if _, err := admin.Exec(ctx, "TRUNCATE guarded_heads, guarded_series"); err != nil {
			t.Fatalf("round %d: TRUNCATE = %v", n, err)
		}
		seedSeries(t, ctx, pool)
		r := raceOnce(t, ctx, pool, read(pool), write)
		if r.errA != nil || r.errB != nil {
			t.Fatalf("round %d: A = %v, B = %v, want both committed", n, r.errA, r.errB)
		}
		if r.stale() {
			stale++
		}
	}
	return stale
}

func underTheGuard(*pgxpool.Pool) preconditionRead {
	return func(ctx context.Context, tx *postgres.Tx, owner string) ([]entry, error) {
		return byOwner.QueryWithin(ctx, tx, owner)
	}
}

func outsideTheTransaction(pool *pgxpool.Pool) preconditionRead {
	return func(ctx context.Context, _ *postgres.Tx, owner string) ([]entry, error) {
		return byOwner.Query(ctx, postgres.NewReadPool(pool), owner)
	}
}

func TestNoGuardedDecisionCommitsOverAReplacedSeries(t *testing.T) {
	for _, write := range seriesWrites {
		t.Run(write.name, func(t *testing.T) {
			if got := staleRounds(t, underTheGuard, write); got != 0 {
				t.Fatalf("stale rounds = %d of %d, want none", got, guardRounds)
			}
		})
	}
}

func TestAReadOutsideTheTransactionCommitsOverAReplacedSeries(t *testing.T) {
	for _, write := range seriesWrites {
		t.Run(write.name, func(t *testing.T) {
			got := staleRounds(t, outsideTheTransaction, write)
			t.Logf("stale rounds = %d of %d", got, guardRounds)
			if got == 0 {
				t.Fatalf("stale rounds = 0 of %d, want the race the guard closes to show at least once", guardRounds)
			}
		})
	}
}
