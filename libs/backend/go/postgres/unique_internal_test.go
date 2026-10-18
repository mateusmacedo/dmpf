package postgres

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// The identity race these fakes stand for depends on the order the server
// inserts into the indexes, which an integration test cannot schedule (ADR-066).

type fakeConn struct {
	pgx.Tx
	tag      pgconn.CommandTag
	open     error
	err      error
	prepare  error
	rollback func(context.Context) error
	found    bool
	batches  [][]string
	execs    []string
	prepared []string
	probes   []string
}

func (c *fakeConn) SendBatch(_ context.Context, b *pgx.Batch) pgx.BatchResults {
	var sent []string
	for _, query := range b.QueuedQueries {
		sent = append(sent, query.SQL)
	}
	c.batches = append(c.batches, sent)
	return &fakeResults{sent: sent, tag: c.tag, open: c.open, err: c.err}
}

func (c *fakeConn) Exec(ctx context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	c.execs = append(c.execs, sql)
	switch {
	case sql == openSavepoint:
		return pgconn.CommandTag{}, c.open
	case c.rollback == nil:
		return pgconn.CommandTag{}, nil
	}
	return pgconn.CommandTag{}, c.rollback(ctx)
}

func (c *fakeConn) Prepare(_ context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	if name != sql {
		return nil, errors.New("prepared under a name other than its text")
	}
	c.prepared = append(c.prepared, sql)
	return &pgconn.StatementDescription{SQL: sql}, c.prepare
}

// batchedTx stands for a connection whose mode never prepares ahead of a batch.
func batchedTx(conn pgx.Tx) *Tx {
	return &Tx{conn: conn, plan: &savepointPlan{unprepared: true}}
}

func (c *fakeConn) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	c.probes = append(c.probes, sql)
	return fakeRow{found: c.found}
}

type fakeRow struct{ found bool }

func (r fakeRow) Scan(...any) error {
	if !r.found {
		return pgx.ErrNoRows
	}
	return nil
}

// pgx answers every result read after a failure with that failure (batch.go:293).
type fakeResults struct {
	pgx.BatchResults
	sent      []string
	tag       pgconn.CommandTag
	open, err error
	failed    error
}

func (r *fakeResults) Exec() (pgconn.CommandTag, error) {
	if r.failed != nil {
		return pgconn.CommandTag{}, r.failed
	}
	sql := r.sent[0]
	r.sent = r.sent[1:]
	switch sql {
	case openSavepoint:
		r.failed = r.open
		return pgconn.CommandTag{}, r.open
	case releaseSavepoint:
		return pgconn.CommandTag{}, nil
	}
	r.failed = r.err
	if r.err != nil {
		return pgconn.CommandTag{}, r.err
	}
	return r.tag, nil
}

func (r *fakeResults) Close() error { return r.failed }

var fakeNameTaken = domain.Reject("probes/duplicate-name", "the name is taken")

var fakeTable = Table[string, string]{
	Name: "unique_probes", IDColumn: "probe_id", Columns: []string{"name"},
	Encode: func(s string) ([]any, error) { return []any{s}, nil },
	Decode: func(func(...any) error) (string, error) { return "", nil },
	Unique: []Unique{{Name: "unique_probes_tenant_id_name_key", Columns: []string{"tenant_id", "name"}, Rejection: fakeNameTaken}},
}

func declaredViolation() *pgconn.PgError {
	return &pgconn.PgError{Code: "23505", TableName: "unique_probes", ConstraintName: "unique_probes_tenant_id_name_key"}
}

func tenantScoped(t *testing.T) context.Context {
	t.Helper()
	subject, tenant := ports.SubjectID("s-1"), ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1", Subject: &subject, Tenant: &tenant,
		Permissions: []ports.Permission{}, Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return ports.WithExecutionContext(context.Background(), execution)
}

func TestACreateOverADeclaredConstraintProbesTheIdentity(t *testing.T) {
	t.Run("the identity is taken", func(t *testing.T) {
		conn := &fakeConn{err: declaredViolation(), found: true}

		err := fakeTable.Repository(batchedTx(conn)).Save(tenantScoped(t), "U-1", "alpha", 0)

		if !errors.Is(err, ports.ErrVersionConflict) || errors.Is(err, ports.ErrUniqueViolation) {
			t.Fatalf("Save() = %v, want ErrVersionConflict: a concurrent create of the same identity is a lost update, never a refusal", err)
		}
		if len(conn.probes) != 1 || conn.probes[0] != fakeTable.compile().exists {
			t.Fatalf("probes = %q, want the one compiled identity probe", conn.probes)
		}
	})
	t.Run("the identity is free", func(t *testing.T) {
		conn := &fakeConn{err: declaredViolation()}

		err := fakeTable.Repository(batchedTx(conn)).Save(tenantScoped(t), "U-1", "alpha", 0)

		if violation, ok := errors.AsType[ports.UniqueViolation](err); !ok || violation.Rejection != fakeNameTaken {
			t.Fatalf("Save() = %v, want the declared rejection", err)
		}
	})
	t.Run("an update does not probe", func(t *testing.T) {
		conn := &fakeConn{err: declaredViolation(), found: true}

		err := fakeTable.Repository(batchedTx(conn)).Save(tenantScoped(t), "U-1", "alpha", 3)

		if violation, ok := errors.AsType[ports.UniqueViolation](err); !ok || violation.Rejection != fakeNameTaken {
			t.Fatalf("Save() = %v, want the declared rejection", err)
		}
		if len(conn.probes) != 0 {
			t.Fatalf("probes = %q, want none: an update cannot race on an identity it already holds", conn.probes)
		}
	})
}

func TestTheSavepointRollbackOutlivesTheCallersCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(tenantScoped(t))
	cancel()
	var (
		undone, bounded bool
		undoErr         error
	)
	conn := &fakeConn{err: declaredViolation(), rollback: func(ctx context.Context) error {
		undone, undoErr = true, ctx.Err()
		_, bounded = ctx.Deadline()
		return nil
	}}

	err := fakeTable.Repository(batchedTx(conn)).Save(ctx, "U-1", "alpha", 3)

	if _, ok := errors.AsType[ports.UniqueViolation](err); !ok {
		t.Fatalf("Save() = %v, want the violation", err)
	}
	if !undone || undoErr != nil {
		t.Fatalf("savepoint rollback ran %v with %v; on the cancelled context it would fail exactly when it has to run", undone, undoErr)
	}
	if !bounded {
		t.Fatal("the savepoint rollback has no deadline; a stuck server would hold the caller forever")
	}
}

func TestASavepointRollbackThatTimesOutJoinsTheOriginalError(t *testing.T) {
	previous := savepointCleanup
	savepointCleanup = 10 * time.Millisecond
	t.Cleanup(func() { savepointCleanup = previous })
	conn := &fakeConn{err: declaredViolation(), rollback: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}

	err := fakeTable.Repository(batchedTx(conn)).Save(tenantScoped(t), "U-1", "alpha", 3)

	var pgErr *pgconn.PgError
	if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("Save() = %v, want the rollback timeout joined to the original 23505", err)
	}
	if errors.Is(err, ports.ErrUniqueViolation) {
		t.Fatalf("Save() = %v, want no translation of a violation whose savepoint was not undone", err)
	}
}

func TestASaveSendsItsSavepointStatementAndReleaseAsOneBatch(t *testing.T) {
	conn := &fakeConn{tag: pgconn.NewCommandTag("INSERT 0 1")}
	tx := batchedTx(conn)

	if err := fakeTable.Repository(tx).Save(tenantScoped(t), "U-1", "alpha", 0); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	if len(conn.batches) != 1 || len(conn.execs) != 0 {
		t.Fatalf("batches = %q, execs = %q; want the whole Save in one batch", conn.batches, conn.execs)
	}
	if want := []string{"SAVEPOINT dmpf_unique", fakeTable.compile().insert, "RELEASE SAVEPOINT dmpf_unique"}; !slices.Equal(conn.batches[0], want) {
		t.Fatalf("batch = %q, want %q", conn.batches[0], want)
	}
	if tx.writes != 1 {
		t.Fatalf("writes = %d, want 1", tx.writes)
	}
}

func TestASaveThatAffectsNoRowIsAVersionConflictThatCountsNoWrite(t *testing.T) {
	for tag, expected := range map[string]ports.Version{"INSERT 0 0": 0, "UPDATE 0": 3} {
		t.Run(tag, func(t *testing.T) {
			tx := batchedTx(&fakeConn{tag: pgconn.NewCommandTag(tag)})

			err := fakeTable.Repository(tx).Save(tenantScoped(t), "U-1", "alpha", expected)

			if !errors.Is(err, ports.ErrVersionConflict) || tx.writes != 0 {
				t.Fatalf("Save() = %v with %d writes, want ErrVersionConflict and none", err, tx.writes)
			}
		})
	}
}

func TestEverySaveOpensAndUndoesTheOneSavepointName(t *testing.T) {
	conn := &fakeConn{err: declaredViolation()}
	repo := fakeTable.Repository(batchedTx(conn))

	for _, id := range []string{"U-1", "U-2"} {
		if _, ok := errors.AsType[ports.UniqueViolation](repo.Save(tenantScoped(t), id, "alpha", 3)); !ok {
			t.Fatalf("Save(%s) did not report the violation", id)
		}
	}

	for _, sent := range conn.batches {
		if sent[0] != "SAVEPOINT dmpf_unique" {
			t.Fatalf("batch = %q, want the fixed name, apart from pgx's sp_N", sent)
		}
	}
	if want := []string{"ROLLBACK TO SAVEPOINT dmpf_unique", "ROLLBACK TO SAVEPOINT dmpf_unique"}; len(conn.batches) != 2 || !slices.Equal(conn.execs, want) {
		t.Fatalf("batches = %q, execs = %q, want two batches each undone by %q", conn.batches, conn.execs, want)
	}
}

func TestASavepointThatFailsToOpenIsNotUndone(t *testing.T) {
	tests := []struct {
		name      string
		open      error
		savepoint bool
	}{
		{"the server refused the SAVEPOINT", &pgconn.PgError{Code: "25P02"}, true},
		{"the SAVEPOINT failed before its result", context.DeadlineExceeded, false},
		{"the caller was cancelled", context.Canceled, false},
		{"the connection was closed", errors.New("conn closed"), false},
	}
	paths := map[string]func(*fakeConn) *Tx{
		"batched with the write": func(conn *fakeConn) *Tx { return batchedTx(conn) },
		"sent alone": func(conn *fakeConn) *Tx {
			return &Tx{conn: conn, plan: &savepointPlan{pins: true, pinned: map[string]bool{}}}
		},
	}
	for path, txOf := range paths {
		for _, tt := range tests {
			t.Run(path+"/"+tt.name, func(t *testing.T) {
				conn := &fakeConn{open: tt.open}

				err := fakeTable.Repository(txOf(conn)).Save(tenantScoped(t), "U-1", "alpha", 3)

				if !errors.Is(err, tt.open) || strings.Contains(err.Error(), "savepoint") != tt.savepoint {
					t.Fatalf("Save() = %v, want %v reported as the savepoint's own failure: %v", err, tt.open, tt.savepoint)
				}
				if slices.Contains(conn.execs, rollbackSavepoint) || len(conn.prepared) != 0 {
					t.Fatalf("execs = %q, prepared = %q, want no rollback of a savepoint not known to be open", conn.execs, conn.prepared)
				}
			})
		}
	}
}

func TestAWriteNotYetPinnedRunsAfterTheSavepointOpensAlone(t *testing.T) {
	conn := &fakeConn{tag: pgconn.NewCommandTag("INSERT 0 1")}
	tx := &Tx{conn: conn, plan: &savepointPlan{pins: true, pinned: map[string]bool{}}}
	insert := fakeTable.compile().insert

	for _, id := range []string{"U-1", "U-2"} {
		if err := fakeTable.Repository(tx).Save(tenantScoped(t), id, id, 0); err != nil {
			t.Fatalf("Save(%s) = %v", id, err)
		}
	}

	if want := []string{"SAVEPOINT dmpf_unique"}; !slices.Equal(conn.execs, want) {
		t.Fatalf("execs = %q, want the first Save to open the savepoint alone", conn.execs)
	}
	if want := []string{insert, "SAVEPOINT dmpf_unique", "RELEASE SAVEPOINT dmpf_unique"}; !slices.Equal(conn.prepared, want) {
		t.Fatalf("prepared = %q, want the write and the savepoint statements pinned once", conn.prepared)
	}
	want := [][]string{{insert, "RELEASE SAVEPOINT dmpf_unique"}, {"SAVEPOINT dmpf_unique", insert, "RELEASE SAVEPOINT dmpf_unique"}}
	if !slices.EqualFunc(conn.batches, want, slices.Equal) {
		t.Fatalf("batches = %q, want %q", conn.batches, want)
	}
}

func TestAWriteThatFailsToPinIsUndoneAndReportedAsTheWrite(t *testing.T) {
	refused := &pgconn.PgError{Code: "42703"}
	conn := &fakeConn{prepare: refused}
	tx := &Tx{conn: conn, plan: &savepointPlan{pins: true, pinned: map[string]bool{}}}

	err := fakeTable.Repository(tx).Save(tenantScoped(t), "U-1", "alpha", 0)

	if !errors.Is(err, refused) || strings.Contains(err.Error(), "savepoint") {
		t.Fatalf("Save() = %v, want the write's own failure", err)
	}
	if want := []string{"SAVEPOINT dmpf_unique", "ROLLBACK TO SAVEPOINT dmpf_unique"}; !slices.Equal(conn.execs, want) || len(conn.batches) != 0 {
		t.Fatalf("execs = %q, batches = %q, want the savepoint opened, undone, and no batch", conn.execs, conn.batches)
	}
	if tx.plan.pinned[fakeTable.compile().insert] {
		t.Fatal("a write that failed to prepare is marked pinned")
	}
}
