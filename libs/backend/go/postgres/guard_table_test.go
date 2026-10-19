//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

type ownerID string

var supersededTwice = domain.Reject("series/superseded-twice", "the entry is already superseded")

var uniqueSeriesTable = func() postgres.Table[string, entry] {
	t := seriesTable
	t.Unique = []postgres.Unique{{
		Name:      "guarded_series_tenant_id_supersedes_id_key",
		Columns:   []string{"tenant_id", "supersedes_id"},
		Rejection: supersededTwice,
	}}
	return t
}()

var byOwner = seriesTable.Relation("owner_id")

func seedEntries(t *testing.T, ctx context.Context, pool *pgxpool.Pool, entries map[string]entry) {
	t.Helper()
	tx := begin(t, pool)
	for _, id := range slices.Sorted(maps.Keys(entries)) {
		if err := seriesTable.Repository(tx).Save(ctx, id, entries[id], 0); err != nil {
			t.Fatalf("seed %s = %v", id, err)
		}
	}
	if err := postgres.ConnOf(tx).Commit(ctx); err != nil {
		t.Fatalf("seed Commit() = %v", err)
	}
}

func heldGuardModes(t *testing.T, admin *pgxpool.Pool, pid uint32) []string {
	t.Helper()
	rows, err := admin.Query(context.Background(),
		"SELECT mode FROM pg_locks WHERE pid = $1 AND locktype = 'advisory' AND granted ORDER BY mode", int64(pid))
	if err != nil {
		t.Fatalf("pg_locks of pid %d = %v", pid, err)
	}
	defer rows.Close()
	var modes []string
	for rows.Next() {
		var mode string
		if err := rows.Scan(&mode); err != nil {
			t.Fatalf("scan mode = %v", err)
		}
		modes = append(modes, mode)
	}
	return modes
}

func requireConflict(t *testing.T, err error, who string) {
	t.Helper()
	coded, ok := errors.AsType[*pgconn.PgError](err)
	if !errors.Is(err, ports.ErrVersionConflict) || !ok || coded.Code != "40P01" {
		t.Fatalf("%s = %v, want ErrVersionConflict over the deadlock it lost", who, err)
	}
}

func TestQueryWithinReadsWhatQueryReadsUnderASharedGuard(t *testing.T) {
	pool := openPool(t)
	admin := openAdminPool(t)
	ctx := scopedTo(t, "acme")
	seedEntries(t, ctx, pool, map[string]entry{
		"E-1": {Owner: "O-1", Status: "enabled"},
		"E-2": {Owner: "O-1", Status: "declared", Supersedes: "E-1"},
		"E-3": {Owner: "O-2", Status: "enabled"},
	})

	outside, err := byOwner.Query(ctx, postgres.NewReadPool(pool), "O-1")
	if err != nil {
		t.Fatalf("Query() = %v", err)
	}
	tx := begin(t, pool)
	inside, err := byOwner.QueryWithin(ctx, tx, ownerID("O-1"))
	if err != nil {
		t.Fatalf("QueryWithin() = %v", err)
	}

	byID := func(a, b entry) int { return strings.Compare(a.ID, b.ID) }
	slices.SortFunc(outside, byID)
	slices.SortFunc(inside, byID)
	if !slices.Equal(inside, outside) || len(inside) != 2 {
		t.Fatalf("QueryWithin() = %+v, want what Query() reads: %+v", inside, outside)
	}
	if modes := heldGuardModes(t, admin, postgres.ConnOf(tx).Conn().PgConn().PID()); !slices.Equal(modes, []string{"ShareLock"}) {
		t.Fatalf("advisory locks held = %v, want the one shared guard", modes)
	}
}

func TestQueryWithinOfAnAbsentValueKeepsTheGuardAndReportsAnotherTenant(t *testing.T) {
	pool := openPool(t)
	admin := openAdminPool(t)
	seedEntries(t, scopedTo(t, "beta"), pool, map[string]entry{"E-9": {Owner: "O-9", Status: "enabled"}})
	ctx := scopedTo(t, "acme")
	tx := begin(t, pool)

	none, err := byOwner.QueryWithin(ctx, tx, "O-1")
	if err != nil || none != nil {
		t.Fatalf("QueryWithin() of a value no tenant holds = %v, %v, want nothing", none, err)
	}
	_, err = byOwner.QueryWithin(ctx, tx, "O-9")
	if access, ok := errors.AsType[ports.CrossTenantAccess](err); !ok || access.DataTenant != "beta" {
		t.Fatalf("QueryWithin() of beta's value = %v, want CrossTenantAccess naming beta", err)
	}
	if modes := heldGuardModes(t, admin, postgres.ConnOf(tx).Conn().PgConn().PID()); len(modes) != 2 {
		t.Fatalf("advisory locks held = %v, want one per value read", modes)
	}
}

func TestASaveOfAUniqueGuardedTableWaitsForTheRead(t *testing.T) {
	for _, path := range []struct {
		name   string
		warmup bool
	}{{"preparing on its connection", false}, {"inlined in the batch", true}} {
		t.Run(path.name, func(t *testing.T) {
			pool := openConcurrencyPool(t)
			writers := openPoolWith(t, func(cfg *pgxpool.Config) { cfg.MaxConns = 1 })
			admin := openAdminPool(t)
			ctx := scopedTo(t, "acme")
			seedEntries(t, ctx, pool, map[string]entry{"E-1": {Owner: "O-1", Status: "enabled"}})
			if path.warmup {
				warm := begin(t, writers)
				if err := uniqueSeriesTable.Repository(warm).Save(ctx, "W-1", entry{Owner: "O-W", Status: "enabled"}, 0); err != nil {
					t.Fatalf("warmup Save() = %v", err)
				}
				_ = postgres.ConnOf(warm).Rollback(ctx)
			}

			reader := begin(t, pool)
			if _, err := byOwner.QueryWithin(ctx, reader, "O-1"); err != nil {
				t.Fatalf("QueryWithin() = %v", err)
			}
			writer := begin(t, writers)
			pid := postgres.ConnOf(writer).Conn().PgConn().PID()
			done := commitAsync(ctx, writer, func() error {
				return uniqueSeriesTable.Repository(writer).Save(ctx, "E-2", entry{Owner: "O-1", Status: "declared", Supersedes: "E-1"}, 0)
			})

			awaitLockWait(t, admin, pid)
			if err := postgres.ConnOf(reader).Commit(ctx); err != nil {
				t.Fatalf("reader Commit() = %v", err)
			}
			if err := <-done; err != nil {
				t.Fatalf("writer = %v, want it committed after the read", err)
			}
		})
	}
}

func TestTheGuardOutlivesTheSavepointARefusedWriteRollsBackTo(t *testing.T) {
	for _, path := range []struct {
		name   string
		warmup bool
	}{{"preparing on its connection", false}, {"inlined in the batch", true}} {
		t.Run(path.name, func(t *testing.T) {
			pool := openConcurrencyPool(t)
			writers := openPoolWith(t, func(cfg *pgxpool.Config) { cfg.MaxConns = 1 })
			admin := openAdminPool(t)
			ctx := scopedTo(t, "acme")
			seedEntries(t, ctx, pool, map[string]entry{
				"E-1": {Owner: "O-1", Status: "enabled"},
				"E-2": {Owner: "O-1", Status: "declared", Supersedes: "E-1"},
			})
			if path.warmup {
				warm := begin(t, writers)
				if err := uniqueSeriesTable.Repository(warm).Save(ctx, "W-1", entry{Owner: "O-W", Status: "enabled"}, 0); err != nil {
					t.Fatalf("warmup Save() = %v", err)
				}
				_ = postgres.ConnOf(warm).Rollback(ctx)
			}

			writer := begin(t, writers)
			err := uniqueSeriesTable.Repository(writer).Save(ctx, "E-3", entry{Owner: "O-1", Status: "declared", Supersedes: "E-1"}, 0)
			if violation, ok := errors.AsType[ports.UniqueViolation](err); !ok || violation.Rejection != supersededTwice {
				t.Fatalf("Save() of a second superseding entry = %v, want the declared violation", err)
			}

			reader := begin(t, pool)
			pid := postgres.ConnOf(reader).Conn().PgConn().PID()
			done := commitAsync(ctx, reader, func() error {
				_, err := byOwner.QueryWithin(ctx, reader, "O-1")
				return err
			})
			awaitLockWait(t, admin, pid)
			if err := postgres.ConnOf(writer).Commit(ctx); err != nil {
				t.Fatalf("writer Commit() = %v", err)
			}
			if err := <-done; err != nil {
				t.Fatalf("reader = %v", err)
			}
		})
	}
}

func TestAnUpdateThatMovesTheGuardIsAVersionConflict(t *testing.T) {
	pool := openPool(t)
	ctx := scopedTo(t, "acme")
	seedEntries(t, ctx, pool, map[string]entry{"E-1": {Owner: "O-1", Status: "enabled"}})
	tx := begin(t, pool)

	err := seriesTable.Repository(tx).Save(ctx, "E-1", entry{Owner: "O-2", Status: "enabled"}, 1)

	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Save() moving E-1 to O-2 = %v, want ErrVersionConflict", err)
	}
}

func TestAGuardOfAnotherTenantDoesNotMakeTheReadWait(t *testing.T) {
	pool := openConcurrencyPool(t)
	seedEntries(t, scopedTo(t, "acme"), pool, map[string]entry{"E-1": {Owner: "O-1", Status: "enabled"}})
	beta := scopedTo(t, "beta")
	writer := begin(t, pool)
	if err := seriesTable.Repository(writer).Save(beta, "E-1", entry{Owner: "O-1", Status: "enabled"}, 0); err != nil {
		t.Fatalf("beta Save() = %v", err)
	}

	ctx, cancel := context.WithTimeout(scopedTo(t, "acme"), 2*time.Second)
	defer cancel()
	if _, err := byOwner.QueryWithin(ctx, begin(t, pool), "O-1"); err != nil {
		t.Fatalf("acme QueryWithin() = %v, want it not to wait for beta's guard", err)
	}
}

func TestTwoReadersThatBothWriteTheSeriesEndInOneConflict(t *testing.T) {
	pool := openConcurrencyPool(t)
	ctx := scopedTo(t, "acme")
	seedEntries(t, ctx, pool, map[string]entry{"E-1": {Owner: "O-1", Status: "enabled"}})

	a, b := begin(t, pool), begin(t, pool)
	for name, tx := range map[string]*postgres.Tx{"A": a, "B": b} {
		if _, err := byOwner.QueryWithin(ctx, tx, "O-1"); err != nil {
			t.Fatalf("%s QueryWithin() = %v", name, err)
		}
	}
	doneA := commitAsync(ctx, a, func() error {
		return seriesTable.Repository(a).Save(ctx, "E-2", entry{Owner: "O-1", Status: "declared", Supersedes: "E-1"}, 0)
	})
	doneB := commitAsync(ctx, b, func() error {
		return seriesTable.Repository(b).Save(ctx, "E-3", entry{Owner: "O-1", Status: "suspended"}, 0)
	})

	errs := []error{<-doneA, <-doneB}
	failed := slices.IndexFunc(errs, func(err error) bool { return err != nil })
	if failed < 0 || errs[1-failed] != nil {
		t.Fatalf("A, B = %v, want exactly one of them to lose the deadlock", errs)
	}
	requireConflict(t, errs[failed], "the loser")
}

func TestADeadlockOnTheRowOfAGuardedTransactionIsAConflict(t *testing.T) {
	pool := openConcurrencyPool(t)
	admin := openAdminPool(t)
	ctx := scopedTo(t, "acme")
	seedSeries(t, ctx, pool)

	reader := begin(t, pool)
	if _, err := byOwner.QueryWithin(ctx, reader, "O-1"); err != nil {
		t.Fatalf("QueryWithin() = %v", err)
	}
	writer := begin(t, pool)
	if err := activate(ctx, writer, "H-1"); err != nil {
		t.Fatalf("writer activate = %v", err)
	}
	readerPID := postgres.ConnOf(reader).Conn().PgConn().PID()
	doneReader := commitAsync(ctx, reader, func() error { return activate(ctx, reader, "H-1") })
	awaitLockWait(t, admin, readerPID)
	doneWriter := commitAsync(ctx, writer, func() error {
		return seriesTable.Repository(writer).Save(ctx, "E-2", entry{Owner: "O-1", Status: "declared", Supersedes: "E-1"}, 0)
	})

	errReader, errWriter := <-doneReader, <-doneWriter
	if errWriter != nil {
		t.Fatalf("writer = %v, want it to commit once the reader lost", errWriter)
	}
	requireConflict(t, errReader, "the reader's UPDATE of H-1")
}

func TestADeadlockWithoutAGuardKeepsTheDriverError(t *testing.T) {
	pool := openConcurrencyPool(t)
	admin := openAdminPool(t)
	ctx := scopedTo(t, "acme")
	seedSeries(t, ctx, pool)
	tx := begin(t, pool)
	if err := headTable.Repository(tx).Save(ctx, "H-2", head{Stage: "homologating"}, 0); err != nil {
		t.Fatalf("seed H-2 = %v", err)
	}
	if err := postgres.ConnOf(tx).Commit(ctx); err != nil {
		t.Fatalf("seed Commit() = %v", err)
	}

	a, b := begin(t, pool), begin(t, pool)
	if err := activate(ctx, a, "H-1"); err != nil {
		t.Fatalf("A H-1 = %v", err)
	}
	if err := activate(ctx, b, "H-2"); err != nil {
		t.Fatalf("B H-2 = %v", err)
	}
	pidA := postgres.ConnOf(a).Conn().PgConn().PID()
	doneA := commitAsync(ctx, a, func() error { return activate(ctx, a, "H-2") })
	awaitLockWait(t, admin, pidA)
	doneB := commitAsync(ctx, b, func() error { return activate(ctx, b, "H-1") })

	errA, errB := <-doneA, <-doneB
	if errB != nil {
		t.Fatalf("B = %v, want it to commit once A lost", errB)
	}
	if coded, ok := errors.AsType[*pgconn.PgError](errA); !ok || coded.Code != "40P01" || errors.Is(errA, ports.ErrVersionConflict) {
		t.Fatalf("A = %v, want the driver's deadlock untouched: no guard was taken", errA)
	}
}

func TestTheGuardCostsNoExtraRoundTrip(t *testing.T) {
	pool, writes := wireCountedPool(t)
	ctx := scopedTo(t, "acme")
	seedEntries(t, ctx, pool, map[string]entry{"E-1": {Owner: "O-1", Status: "enabled"}})

	tests := []struct {
		name string
		run  func(ctx context.Context, tx *postgres.Tx) error
		want int64
	}{
		{"a read of a held value", func(ctx context.Context, tx *postgres.Tx) error {
			_, err := byOwner.QueryWithin(ctx, tx, "O-1")
			return err
		}, 1},
		{"a read of an absent value, probing its owner", func(ctx context.Context, tx *postgres.Tx) error {
			_, err := byOwner.QueryWithin(ctx, tx, "O-0")
			return err
		}, 2},
		{"a guarded create", func(ctx context.Context, tx *postgres.Tx) error {
			return seriesTable.Repository(tx).Save(ctx, "E-2", entry{Owner: "O-1", Status: "declared", Supersedes: "E-1"}, 0)
		}, 1},
		{"a guarded update", func(ctx context.Context, tx *postgres.Tx) error {
			return seriesTable.Repository(tx).Save(ctx, "E-1", entry{Owner: "O-1", Status: "suspended"}, 1)
		}, 1},
		{"a guarded create under the savepoint", func(ctx context.Context, tx *postgres.Tx) error {
			return uniqueSeriesTable.Repository(tx).Save(ctx, "E-2", entry{Owner: "O-1", Status: "declared", Supersedes: "E-1"}, 0)
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []int64
			for range 2 {
				conn, err := pool.Begin(ctx)
				if err != nil {
					t.Fatalf("Begin() = %v", err)
				}
				before := writes.Load()
				if err := tt.run(ctx, postgres.NewTx(conn)); err != nil {
					t.Fatalf("run = %v", err)
				}
				got = append(got, writes.Load()-before)
				if err := conn.Rollback(ctx); err != nil {
					t.Fatalf("Rollback() = %v", err)
				}
			}
			if got[1] != tt.want {
				t.Fatalf("round trips on a warm connection = %d (cold %d), want %d", got[1], got[0], tt.want)
			}
		})
	}
}

func TestAGuardedSaveAfterARefusedOneStillCostsOneRoundTrip(t *testing.T) {
	pool, writes := wireCountedPool(t)
	ctx := scopedTo(t, "acme")
	seedEntries(t, ctx, pool, map[string]entry{
		"E-1": {Owner: "O-1", Status: "enabled"},
		"E-2": {Owner: "O-1", Status: "declared", Supersedes: "E-1"},
	})
	save := func(id, supersedes string) (int64, error) {
		conn, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin() = %v", err)
		}
		defer func() { _ = conn.Rollback(ctx) }()
		before := writes.Load()
		err = uniqueSeriesTable.Repository(postgres.NewTx(conn)).Save(ctx, id, entry{Owner: "O-1", Status: "declared", Supersedes: supersedes}, 0)
		return writes.Load() - before, err
	}
	for range 2 {
		if _, err := save("E-3", "E-2"); err != nil {
			t.Fatalf("warmup Save() = %v", err)
		}
	}
	if _, err := save("E-4", "E-1"); !errors.As(err, new(ports.UniqueViolation)) {
		t.Fatalf("refused Save() = %v, want the declared violation", err)
	}

	if got, err := save("E-3", "E-2"); err != nil || got != 1 {
		t.Fatalf("Save() after the refusal = %d round trips, %v; want 1: the batch that failed must not evict the guard's statement", got, err)
	}
}

func TestAStatementTheServerRefusesToPrepareIsNotBlamedOnTheGuard(t *testing.T) {
	pool := openPool(t)
	ctx := scopedTo(t, "acme")
	ghostly := seriesTable
	ghostly.Columns = []string{"owner_id", "status", "ghost"}
	ghostly.Encode = func(e entry) ([]any, error) { return []any{e.Owner, e.Status, "boo"}, nil }

	_, readErr := ghostly.Relation("owner_id").QueryWithin(ctx, begin(t, pool), "O-1")
	saveErr := ghostly.Repository(begin(t, pool)).Save(ctx, "E-1", entry{Owner: "O-1", Status: "enabled"}, 0)

	for name, err := range map[string]error{"QueryWithin": readErr, "Save": saveErr} {
		coded, ok := errors.AsType[*pgconn.PgError](err)
		if !ok || coded.Code != "42703" || strings.Contains(err.Error(), ": guard") {
			t.Errorf("%s = %v, want the undefined column reported on its own statement, not on the guard", name, err)
		}
	}
}

func TestADeadlockInsideTheSavepointOfAGuardedSaveIsAConflict(t *testing.T) {
	pool := openConcurrencyPool(t)
	admin := openAdminPool(t)
	ctx := scopedTo(t, "acme")

	other := begin(t, pool)
	if err := uniqueSeriesTable.Repository(other).Save(ctx, "E-20", entry{Owner: "O-2", Status: "declared", Supersedes: "S"}, 0); err != nil {
		t.Fatalf("other Save() = %v", err)
	}
	victim := begin(t, pool)
	pid := postgres.ConnOf(victim).Conn().PgConn().PID()
	doneVictim := commitAsync(ctx, victim, func() error {
		return uniqueSeriesTable.Repository(victim).Save(ctx, "E-10", entry{Owner: "O-1", Status: "declared", Supersedes: "S"}, 0)
	})
	awaitLockWait(t, admin, pid)
	doneOther := commitAsync(ctx, other, func() error {
		_, err := byOwner.QueryWithin(ctx, other, "O-1")
		return err
	})

	errVictim, errOther := <-doneVictim, <-doneOther
	if errOther != nil {
		t.Fatalf("other = %v, want it to commit once the victim lost", errOther)
	}
	requireConflict(t, errVictim, "the INSERT waiting on the other's unique key")
}

func TestTenantsWhoseTextsConcatenateAlikeTakeDifferentGuards(t *testing.T) {
	pool := openConcurrencyPool(t)
	writer := begin(t, pool)
	if err := seriesTable.Repository(writer).Save(scopedTo(t, "t\x1fa"), "E-1", entry{Owner: "b", Status: "enabled"}, 0); err != nil {
		t.Fatalf("Save() in tenant t\\x1fa = %v", err)
	}

	ctx, cancel := context.WithTimeout(scopedTo(t, "t"), 2*time.Second)
	defer cancel()
	if _, err := byOwner.QueryWithin(ctx, begin(t, pool), "a\x1fb"); err != nil {
		t.Fatalf("QueryWithin(a\\x1fb) in tenant t = %v, want no wait on tenant t\\x1fa's guard of b", err)
	}
}
