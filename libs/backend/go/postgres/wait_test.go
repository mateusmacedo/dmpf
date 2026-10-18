//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	lognoop "go.opentelemetry.io/otel/log/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func TestWaitForTablesReturnsOnceEveryTableExists(t *testing.T) {
	pool := openPool(t)
	admin := openAdminPool(t)
	ctx := context.Background()
	if _, err := admin.Exec(ctx, "DROP TABLE IF EXISTS late_probes"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP TABLE IF EXISTS late_probes") })

	done := make(chan error, 1)
	go func() {
		done <- postgres.WaitForTables(ctx, pool, 20*time.Millisecond, lognoop.NewLoggerProvider(), "outbox", "late_probes")
	}()

	select {
	case err := <-done:
		t.Fatalf("WaitForTables() = %v before late_probes existed", err)
	case <-time.After(150 * time.Millisecond):
	}
	if _, err := admin.Exec(ctx, "CREATE TABLE late_probes (id bigint)"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitForTables() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForTables() did not return after the table was created")
	}
}

func TestWaitForTablesStopsWithTheContext(t *testing.T) {
	pool := openPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := postgres.WaitForTables(ctx, pool, 20*time.Millisecond, lognoop.NewLoggerProvider(), "never_created")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForTables() = %v, want context.DeadlineExceeded", err)
	}
}

func TestWaitForTablesNamesTheTableItWaitsFor(t *testing.T) {
	pool := openPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	logs := &recordingProcessor{}
	_ = postgres.WaitForTables(ctx, pool, 20*time.Millisecond, logs.provider(), "never_created")

	records := logs.named("waiting for table")
	if len(records) != 1 || records[0]["db.collection.name"] != "never_created" || records[0]["scope"] != postgresScope {
		t.Fatalf("log = %v, want one record naming never_created under db.collection.name, scoped to %s", records, postgresScope)
	}
	if _, free := records[0]["table"]; free {
		t.Fatalf("log = %v still carries the free key table", records)
	}
}

func TestWaitForGrantsReturnsOnceThePrivilegeIsGranted(t *testing.T) {
	admin := openAdminPool(t)
	ctx := context.Background()
	ensureRole(t, admin, grantsRuntime, safeRuntime)
	runtime := openRuntime(t, grantsRuntime)

	done := make(chan error, 1)
	go func() {
		done <- postgres.WaitForGrants(ctx, runtime, 20*time.Millisecond, lognoop.NewLoggerProvider(), probeGrants()...)
	}()

	select {
	case err := <-done:
		t.Fatalf("WaitForGrants() = %v before the grants existed", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := postgres.ApplyGrants(ctx, admin, grantsRuntime, probeGrants()...); err != nil {
		t.Fatalf("ApplyGrants() = %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitForGrants() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForGrants() did not return after the grants were applied")
	}
}

func TestWaitForGrantsReturnsForTheOwnerOfTheTables(t *testing.T) {
	pool := openAdminPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := postgres.WaitForGrants(ctx, pool, 20*time.Millisecond, lognoop.NewLoggerProvider(), probeGrants()...); err != nil {
		t.Fatalf("WaitForGrants() by the owner = %v, want nil", err)
	}
}

func TestWaitForGrantsWaitsForAnAbsentTable(t *testing.T) {
	pool := openAdminPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS late_probes"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP TABLE IF EXISTS late_probes") })

	done := make(chan error, 1)
	go func() {
		done <- postgres.WaitForGrants(ctx, pool, 20*time.Millisecond, lognoop.NewLoggerProvider(), postgres.AppendOnly("late_probes"))
	}()

	select {
	case err := <-done:
		t.Fatalf("WaitForGrants() = %v before late_probes existed", err)
	case <-time.After(150 * time.Millisecond):
	}
	if _, err := pool.Exec(ctx, "CREATE TABLE late_probes (id bigint)"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitForGrants() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForGrants() did not return after the table was created")
	}
}

func TestWaitForGrantsStopsWithTheContext(t *testing.T) {
	pool := openPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := postgres.WaitForGrants(ctx, pool, 20*time.Millisecond, lognoop.NewLoggerProvider(), postgres.AppendOnly("never_created"))

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForGrants() = %v, want context.DeadlineExceeded", err)
	}
}

func TestWaitForGrantsNamesTheTableAndThePrivilegeItWaitsFor(t *testing.T) {
	pool := openPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	logs := &recordingProcessor{}

	_ = postgres.WaitForGrants(ctx, pool, 20*time.Millisecond, logs.provider(), postgres.AppendOnly("never_created"))

	records := logs.named("waiting for table privilege")
	if len(records) != 1 || records[0]["db.collection.name"] != "never_created" || records[0]["db.operation.name"] != "SELECT" || records[0]["scope"] != postgresScope {
		t.Fatalf("log = %v, want one record naming never_created and SELECT, scoped to %s", records, postgresScope)
	}
}

const lateUnique = "late_uniques_tenant_id_name_key"

func lateUniques() postgres.UniqueSet {
	return postgres.UniqueSet{Table: "late_uniques", Constraints: []postgres.Unique{{
		Name: lateUnique, Columns: []string{"tenant_id", "name"}, Rejection: nameTaken,
	}}}
}

func createLateUniques(t *testing.T, admin *pgxpool.Pool) {
	t.Helper()
	mustExec(t, admin, "DROP TABLE IF EXISTS late_uniques")
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP TABLE IF EXISTS late_uniques") })
	mustExec(t, admin, "CREATE TABLE late_uniques (tenant_id text NOT NULL, name text NOT NULL)")
}

func TestWaitForUniquesReturnsOnceTheConstraintExists(t *testing.T) {
	pool := openPool(t)
	admin := openAdminPool(t)
	createLateUniques(t, admin)

	done := make(chan error, 1)
	go func() {
		done <- postgres.WaitForUniques(context.Background(), pool, 20*time.Millisecond, lognoop.NewLoggerProvider(), uniqueTable.Uniques(), lateUniques())
	}()

	select {
	case err := <-done:
		t.Fatalf("WaitForUniques() = %v before %s existed", err, lateUnique)
	case <-time.After(150 * time.Millisecond):
	}
	mustExec(t, admin, "ALTER TABLE late_uniques ADD CONSTRAINT "+lateUnique+" UNIQUE (tenant_id, name)")
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitForUniques() = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForUniques() did not return after the constraint was added")
	}
}

func TestWaitForUniquesReturnsAnyOtherErrorAtOnce(t *testing.T) {
	pool := openPool(t)
	malformed := lateUniques()
	malformed.Constraints[0].Rejection = nil
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := postgres.WaitForUniques(ctx, pool, 20*time.Millisecond, lognoop.NewLoggerProvider(), malformed)

	if err == nil || errors.Is(err, postgres.ErrUniqueDivergent) || ctx.Err() != nil {
		t.Fatalf("WaitForUniques() over a malformed declaration = %v, want its error before the deadline", err)
	}
}

func TestWaitForUniquesStopsWithTheContextKeepingTheDivergence(t *testing.T) {
	pool := openPool(t)
	createLateUniques(t, openAdminPool(t))
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	err := postgres.WaitForUniques(ctx, pool, 20*time.Millisecond, lognoop.NewLoggerProvider(), lateUniques())

	if !errors.Is(err, context.Canceled) || !errors.Is(err, postgres.ErrUniqueDivergent) || !strings.Contains(err.Error(), lateUnique) {
		t.Fatalf("WaitForUniques() = %v, want context.Canceled with the ErrUniqueDivergent of %s", err, lateUnique)
	}
}

func TestWaitForUniquesNamesTheConstraintItWaitsFor(t *testing.T) {
	pool := openPool(t)
	createLateUniques(t, openAdminPool(t))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	logs := &recordingProcessor{}

	_ = postgres.WaitForUniques(ctx, pool, 20*time.Millisecond, logs.provider(), lateUniques())

	records := logs.named("waiting for unique constraint")
	if len(records) != 1 || records[0]["db.collection.name"] != "late_uniques" || records[0]["dmpf.db.constraint.name"] != lateUnique ||
		records[0]["dmpf.db.constraint.divergence"] != "absent" || records[0]["scope"] != postgresScope {
		t.Fatalf("log = %v, want one record naming late_uniques, %s and its divergence, scoped to %s", records, lateUnique, postgresScope)
	}
}
