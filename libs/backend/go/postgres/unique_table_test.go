//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

type uniqueProbe struct {
	Name, Code, Label, RefA, RefB string
}

var (
	nameTaken = domain.Reject("probes/duplicate-name", "the name is taken")
	codeTaken = domain.Reject("probes/duplicate-code", "the code is taken")
	refsTaken = domain.Reject("probes/duplicate-refs", "the pair of references is taken")
)

// unique_probes_tenant_id_label_key stays undeclared: it is the constraint the
// schema enforces without the context having given it a rejection.
var uniqueTable = postgres.Table[string, uniqueProbe]{
	Name:     "unique_probes",
	IDColumn: "probe_id",
	Columns:  []string{"name", "code", "label", "ref_a", "ref_b"},
	Encode: func(p uniqueProbe) ([]any, error) {
		return []any{p.Name, p.Code, p.Label, p.RefA, p.RefB}, nil
	},
	Decode: func(scan func(dest ...any) error) (uniqueProbe, error) {
		var p uniqueProbe
		err := scan(&p.Name, &p.Code, &p.Label, &p.RefA, &p.RefB)
		return p, err
	},
	Unique: []postgres.Unique{
		{Name: "unique_probes_tenant_id_name_key", Columns: []string{"tenant_id", "name"}, Rejection: nameTaken},
		{Name: "unique_probes_tenant_id_code_key", Columns: []string{"tenant_id", "code"}, Rejection: codeTaken},
		{Name: "unique_probes_tenant_id_ref_a_ref_b_key", Columns: []string{"tenant_id", "ref_a", "ref_b"}, Rejection: refsTaken},
	},
}

func saveUnique(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string, state uniqueProbe, expected ports.Version) error {
	t.Helper()
	uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) ports.Repository[string, uniqueProbe] {
		return uniqueTable.Repository(tx)
	})
	return uow.Within(ctx, func(ctx context.Context, repo ports.Repository[string, uniqueProbe]) error {
		return repo.Save(ctx, id, state, expected)
	})
}

func TestTheUniqueFixtureRoundTripsUnderTheRuntimeRole(t *testing.T) {
	pool := openPool(t)
	ctx := scopedTo(t, "acme")
	want := uniqueProbe{Name: "alpha", Code: "A", Label: "red", RefA: "A", RefB: "red"}

	if err := saveUnique(t, ctx, pool, "U-1", want, 0); err != nil {
		t.Fatalf("Save() = %v", err)
	}
	got, version, err := uniqueTable.Reader(postgres.NewReadPool(pool)).Load(ctx, "U-1")
	if err != nil || got != want || version != 1 {
		t.Fatalf("Load() = %+v at %d, %v; want %+v at 1", got, version, err, want)
	}
}

func begin(t *testing.T, pool *pgxpool.Pool) *postgres.Tx {
	t.Helper()
	conn, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	t.Cleanup(func() { _ = conn.Rollback(context.Background()) })
	return postgres.NewTx(conn)
}

func requireWrites(t *testing.T, tx *postgres.Tx, want int, after string) {
	t.Helper()
	if got := postgres.WritesOf(tx); got != want {
		t.Fatalf("writes after %s = %d, want %d", after, got, want)
	}
}

func TestATransactionCountsOnlyItsAggregateAndOutboxWrites(t *testing.T) {
	pool := openPool(t)
	ctx := scopedTo(t, "acme")
	tx, other := begin(t, pool), begin(t, pool)

	reception, err := tx.Inbox("orders", 0).Register(ctx, receipt("orders", "m-1", "h1"))
	if err != nil {
		t.Fatalf("Register() = %v", err)
	}
	if err := reception.Match(
		func(p ports.Pending) error {
			return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: 200})
		},
		func() error { return nil }, func() error { return nil }, func() error { return nil },
	); err != nil {
		t.Fatalf("Complete() = %v", err)
	}
	requireWrites(t, tx, 0, "the inbox registered and completed a message")

	repo := probeTable.Repository(tx)
	if err := repo.Save(ctx, "P-1", probe{Items: 1}, 0); err != nil {
		t.Fatalf("Save() = %v", err)
	}
	requireWrites(t, tx, 1, "a Save that affected its row")

	if err := repo.Save(ctx, "P-1", probe{Items: 2}, 7); !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Save() at a stale version = %v, want ErrVersionConflict", err)
	}
	requireWrites(t, tx, 1, "a Save that matched no row")

	outbox := tx.Outbox(testMapper{})
	if err := outbox.Enqueue(ctx, outboxEntry("m-1", placedEvent(1))); err != nil {
		t.Fatalf("Enqueue() = %v", err)
	}
	requireWrites(t, tx, 2, "an Enqueue")

	if err := outbox.Enqueue(ctx, outboxEntry("m-1", placedEvent(1))); !errors.Is(err, postgres.ErrDuplicateMessage) {
		t.Fatalf("Enqueue() of a duplicate = %v, want ErrDuplicateMessage", err)
	}
	requireWrites(t, tx, 2, "a refused Enqueue")
	requireWrites(t, other, 0, "writes on another transaction")
}

func requireViolation(t *testing.T, err error, constraint string, want *domain.Rejection) {
	t.Helper()
	violation, ok := errors.AsType[ports.UniqueViolation](err)
	if !ok || !errors.Is(err, ports.ErrUniqueViolation) {
		t.Fatalf("Save() = %v, want a UniqueViolation", err)
	}
	if violation.Table != "unique_probes" || violation.Constraint != constraint || violation.Rejection != want {
		t.Fatalf("violation = %+v, want %s with rejection %v", violation, constraint, want)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("Save() = %v, want the PgError still reachable with SQLSTATE 23505", err)
	}
}

func seedUnique(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for id, state := range map[string]uniqueProbe{
		"U-1": {Name: "alpha", Code: "A", Label: "red", RefA: "A", RefB: "red"},
		"U-2": {Name: "beta", Code: "B", Label: "blue", RefA: "B", RefB: "blue"},
	} {
		if err := saveUnique(t, ctx, pool, id, state, 0); err != nil {
			t.Fatalf("Save(%s) = %v", id, err)
		}
	}
}

func TestEachDeclaredConstraintReturnsItsOwnRejection(t *testing.T) {
	pool := openPool(t)
	ctx := scopedTo(t, "acme")
	seedUnique(t, ctx, pool)

	tests := []struct {
		name       string
		id         string
		state      uniqueProbe
		expected   ports.Version
		constraint string
		want       *domain.Rejection
	}{
		{"a create over a taken name", "U-3", uniqueProbe{Name: "alpha", Code: "C", Label: "green", RefA: "C", RefB: "green"}, 0, "unique_probes_tenant_id_name_key", nameTaken},
		{"a create over a taken code", "U-3", uniqueProbe{Name: "gamma", Code: "A", Label: "green", RefA: "A", RefB: "green"}, 0, "unique_probes_tenant_id_code_key", codeTaken},
		{"an update onto a taken name", "U-2", uniqueProbe{Name: "alpha", Code: "B", Label: "blue", RefA: "B", RefB: "blue"}, 1, "unique_probes_tenant_id_name_key", nameTaken},
		{"an update onto a taken code", "U-2", uniqueProbe{Name: "beta", Code: "A", Label: "blue", RefA: "A", RefB: "blue"}, 1, "unique_probes_tenant_id_code_key", codeTaken},
		{"a create over a taken pair of references", "U-3", uniqueProbe{Name: "gamma", Code: "C", Label: "green", RefA: "A", RefB: "red"}, 0, "unique_probes_tenant_id_ref_a_ref_b_key", refsTaken},
		{"an update onto a taken pair of references", "U-2", uniqueProbe{Name: "beta", Code: "B", Label: "blue", RefA: "A", RefB: "red"}, 1, "unique_probes_tenant_id_ref_a_ref_b_key", refsTaken},
		{"a create over the undeclared label", "U-3", uniqueProbe{Name: "gamma", Code: "C", Label: "red", RefA: "C", RefB: "red"}, 0, "unique_probes_tenant_id_label_key", nil},
		{"an update onto the undeclared label", "U-2", uniqueProbe{Name: "beta", Code: "B", Label: "red", RefA: "B", RefB: "red"}, 1, "unique_probes_tenant_id_label_key", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireViolation(t, saveUnique(t, ctx, pool, tt.id, tt.state, tt.expected), tt.constraint, tt.want)
		})
	}
}

func TestADeclaredViolationAsTheFirstWriteLeavesTheTransactionUsable(t *testing.T) {
	pool := openPool(t)
	ctx := scopedTo(t, "acme")
	seedUnique(t, ctx, pool)
	tx := begin(t, pool)
	repo := uniqueTable.Repository(tx)

	requireViolation(t, repo.Save(ctx, "U-3", uniqueProbe{Name: "alpha", Code: "C", Label: "green", RefA: "C", RefB: "green"}, 0),
		"unique_probes_tenant_id_name_key", nameTaken)
	requireWrites(t, tx, 0, "a violation the savepoint undid")

	if err := repo.Save(ctx, "U-3", uniqueProbe{Name: "gamma", Code: "C", Label: "green", RefA: "C", RefB: "green"}, 0); err != nil {
		t.Fatalf("Save() after the violation = %v, want the transaction still usable", err)
	}
	if err := postgres.ConnOf(tx).Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v, want the transaction to commit what followed the violation", err)
	}
	got, _, err := uniqueTable.Reader(postgres.NewReadPool(pool)).Load(ctx, "U-3")
	if err != nil || got.Name != "gamma" {
		t.Fatalf("Load(U-3) = %+v, %v; want the write that followed the violation", got, err)
	}
}

func TestAViolationAfterAnotherWriteIsNotConvertible(t *testing.T) {
	for name, write := range map[string]func(ctx context.Context, tx *postgres.Tx) error{
		"after a Save": func(ctx context.Context, tx *postgres.Tx) error {
			return uniqueTable.Repository(tx).Save(ctx, "U-5", uniqueProbe{Name: "epsilon", Code: "E", Label: "black", RefA: "E", RefB: "black"}, 0)
		},
		"after an Enqueue": func(ctx context.Context, tx *postgres.Tx) error {
			return tx.Outbox(testMapper{}).Enqueue(ctx, outboxEntry("m-1", placedEvent(1)))
		},
	} {
		t.Run(name, func(t *testing.T) {
			pool := openPool(t)
			ctx := scopedTo(t, "acme")
			seedUnique(t, ctx, pool)
			tx := begin(t, pool)
			if err := write(ctx, tx); err != nil {
				t.Fatalf("prior write = %v", err)
			}

			err := uniqueTable.Repository(tx).Save(ctx, "U-3", uniqueProbe{Name: "alpha", Code: "C", Label: "green", RefA: "C", RefB: "green"}, 0)

			requireViolation(t, err, "unique_probes_tenant_id_name_key", nil)
		})
	}
}

func TestAnotherTenantMayHoldTheSameUniqueValues(t *testing.T) {
	pool := openPool(t)
	seedUnique(t, scopedTo(t, "acme"), pool)

	if err := saveUnique(t, scopedTo(t, "globex"), pool, "U-1", uniqueProbe{Name: "alpha", Code: "A", Label: "red", RefA: "A", RefB: "red"}, 0); err != nil {
		t.Fatalf("Save() of the same values in another tenant = %v, want nil: every constraint leads with tenant_id", err)
	}
}

func TestADeniedStatementUnderTheSavepointIsReturnedAsItIs(t *testing.T) {
	pool := openPool(t)
	admin := openAdminPool(t)
	ctx := scopedTo(t, "acme")
	seedUnique(t, ctx, pool)

	mustExec(t, admin, "REVOKE UPDATE ON unique_probes FROM "+pg.RuntimeRole)
	err := saveUnique(t, ctx, pool, "U-1", uniqueProbe{Name: "alpha", Code: "A", Label: "pink", RefA: "A", RefB: "pink"}, 1)
	requireDenied(t, err, "Save() of an existing aggregate under the savepoint")
	mustExec(t, admin, "REVOKE INSERT ON unique_probes FROM "+pg.RuntimeRole)
	err = saveUnique(t, ctx, pool, "U-3", uniqueProbe{Name: "gamma", Code: "C", Label: "green", RefA: "C", RefB: "green"}, 0)
	requireDenied(t, err, "Save() of a new aggregate under the savepoint")
	if errors.Is(err, ports.ErrUniqueViolation) {
		t.Fatalf("Save() without privilege = %v, reported as a unique violation", err)
	}
}

func tracedPool(t *testing.T, configure ...func(*pgxpool.Config)) (*pgxpool.Pool, trace.Tracer, *tracetest.InMemoryExporter) {
	t.Helper()
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	tracer := provider.Tracer("unique-test")
	pool := openPoolWith(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = postgres.NewQueryTracer(tracer)
		for _, c := range configure {
			c(cfg)
		}
	})
	return pool, tracer, spans
}

func statementsOf(spans *tracetest.InMemoryExporter, parent trace.Span) []tracetest.SpanStub {
	var out []tracetest.SpanStub
	for _, span := range spans.GetSpans() {
		if span.Parent.SpanID() == parent.SpanContext().SpanID() {
			out = append(out, span)
		}
	}
	return out
}

func TestTheSavepointCostsTwoStatementsAndARejectedCreateThree(t *testing.T) {
	pool, tracer, spans := tracedPool(t)
	ctx := scopedTo(t, "acme")
	seedUnique(t, ctx, pool)

	tests := []struct {
		name string
		save func(ctx context.Context, tx *postgres.Tx) error
		want []string
	}{
		{"a table without Unique", func(ctx context.Context, tx *postgres.Tx) error {
			return taggedTable.Repository(tx).Save(ctx, "T-1", tagged{Label: "blue"}, 0)
		}, []string{"INSERT tagged_probes"}},
		{"a create", func(ctx context.Context, tx *postgres.Tx) error {
			return uniqueTable.Repository(tx).Save(ctx, "U-3", uniqueProbe{Name: "gamma", Code: "C", Label: "green", RefA: "C", RefB: "green"}, 0)
		}, []string{"SAVEPOINT", "INSERT unique_probes", "RELEASE SAVEPOINT"}},
		{"an update", func(ctx context.Context, tx *postgres.Tx) error {
			return uniqueTable.Repository(tx).Save(ctx, "U-1", uniqueProbe{Name: "alpha", Code: "A", Label: "pink", RefA: "A", RefB: "pink"}, 1)
		}, []string{"SAVEPOINT", "UPDATE unique_probes", "RELEASE SAVEPOINT"}},
		{"an update that broke a declared constraint", func(ctx context.Context, tx *postgres.Tx) error {
			return uniqueTable.Repository(tx).Save(ctx, "U-2", uniqueProbe{Name: "alpha", Code: "B", Label: "blue", RefA: "B", RefB: "blue"}, 1)
		}, []string{"SAVEPOINT", "UPDATE unique_probes", "ROLLBACK TO SAVEPOINT"}},
		{"a create that broke a declared constraint", func(ctx context.Context, tx *postgres.Tx) error {
			return uniqueTable.Repository(tx).Save(ctx, "U-4", uniqueProbe{Name: "alpha", Code: "D", Label: "purple", RefA: "D", RefB: "purple"}, 0)
		}, []string{"SAVEPOINT", "INSERT unique_probes", "ROLLBACK TO SAVEPOINT", "SELECT unique_probes"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := begin(t, pool)
			spanCtx, parent := tracer.Start(ctx, "save")
			_ = tt.save(spanCtx, tx)
			parent.End()

			var got []string
			for _, span := range statementsOf(spans, parent) {
				got = append(got, span.Name)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("statements = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTheViolatedValueLeavesNoTrace(t *testing.T) {
	const sentinel = "sentinel-7f3a9c"
	pool, tracer, spans := tracedPool(t)
	ctx := scopedTo(t, "acme")
	if err := saveUnique(t, ctx, pool, "U-1", uniqueProbe{Name: sentinel, Code: "A", Label: "red", RefA: "A", RefB: "red"}, 0); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	tx := begin(t, pool)
	spanCtx, parent := tracer.Start(ctx, "save")
	err := uniqueTable.Repository(tx).Save(spanCtx, "U-2", uniqueProbe{Name: sentinel, Code: "B", Label: "blue", RefA: "B", RefB: "blue"}, 0)
	parent.End()

	requireViolation(t, err, "unique_probes_tenant_id_name_key", nameTaken)
	violation, _ := errors.AsType[ports.UniqueViolation](err)
	rejection := violation.Rejection
	if strings.Contains(err.Error(), sentinel) || strings.Contains(rejection.Error(), sentinel) || slices.ContainsFunc(rejection.Details(), func(d domain.Detail) bool {
		return strings.Contains(d.Value, sentinel)
	}) {
		t.Fatalf("error %q or rejection %v carries the violated value; the DETAIL quotes it and must stay on the server (ERR-07)", err, rejection)
	}
	for _, span := range statementsOf(spans, parent) {
		if strings.Contains(span.Name, sentinel) {
			t.Fatalf("span name %q carries the violated value (ERR-07)", span.Name)
		}
		for _, kv := range span.Attributes {
			if strings.Contains(kv.Value.String(), sentinel) {
				t.Fatalf("span %s attribute %s = %q carries the violated value (ERR-07)", span.Name, kv.Key, kv.Value.String())
			}
		}
	}
}

type countedConn struct {
	net.Conn
	writes *atomic.Int64
}

func (c countedConn) Write(p []byte) (int, error) {
	c.writes.Add(1)
	return c.Conn.Write(p)
}

// One connection, so the statement cache the first run warms serves the measured one.
func wireCountedPool(t *testing.T, configure ...func(*pgxpool.Config)) (*pgxpool.Pool, *atomic.Int64) {
	t.Helper()
	writes := new(atomic.Int64)
	pool := openPoolWith(t, func(cfg *pgxpool.Config) {
		for _, c := range configure {
			c(cfg)
		}
		cfg.MaxConns = 1
		dial := cfg.ConnConfig.DialFunc
		cfg.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dial(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			return countedConn{Conn: conn, writes: writes}, nil
		}
	})
	return pool, writes
}

func TestASaveUnderTheSavepointCostsOneRoundTrip(t *testing.T) {
	pool, writes := wireCountedPool(t)
	ctx := scopedTo(t, "acme")
	seedUnique(t, ctx, pool)

	tests := []struct {
		name    string
		save    func(ctx context.Context, tx *postgres.Tx) error
		warmups int
		want    []int64
	}{
		{"a create", func(ctx context.Context, tx *postgres.Tx) error {
			return uniqueTable.Repository(tx).Save(ctx, "U-3", uniqueProbe{Name: "gamma", Code: "C", Label: "green", RefA: "C", RefB: "green"}, 0)
		}, 1, []int64{1}},
		{"an update", func(ctx context.Context, tx *postgres.Tx) error {
			return uniqueTable.Repository(tx).Save(ctx, "U-1", uniqueProbe{Name: "alpha", Code: "A", Label: "pink", RefA: "A", RefB: "pink"}, 1)
		}, 1, []int64{1}},
		{"a rejected create, the first preparing its probe", func(ctx context.Context, tx *postgres.Tx) error {
			return uniqueTable.Repository(tx).Save(ctx, "U-4", uniqueProbe{Name: "alpha", Code: "D", Label: "purple", RefA: "D", RefB: "purple"}, 0)
		}, 0, []int64{4, 3, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []int64
			for range tt.warmups + len(tt.want) {
				conn, err := pool.Begin(ctx)
				if err != nil {
					t.Fatalf("Begin() = %v", err)
				}
				before := writes.Load()
				_ = tt.save(ctx, postgres.NewTx(conn))
				got = append(got, writes.Load()-before)
				if err := conn.Rollback(ctx); err != nil {
					t.Fatalf("Rollback() = %v", err)
				}
			}
			if got = got[tt.warmups:]; !slices.Equal(got, tt.want) {
				t.Fatalf("round trips = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTheSavepointDoesNotCollideWithTheCallersOwn(t *testing.T) {
	pool := openPool(t)
	ctx := scopedTo(t, "acme")
	seedUnique(t, ctx, pool)
	outer, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	t.Cleanup(func() { _ = outer.Rollback(context.Background()) })
	nested, err := outer.Begin(ctx)
	if err != nil {
		t.Fatalf("nested Begin() = %v", err)
	}
	repo := uniqueTable.Repository(postgres.NewTx(nested))

	requireViolation(t, repo.Save(ctx, "U-3", uniqueProbe{Name: "alpha", Code: "C", Label: "green", RefA: "C", RefB: "green"}, 0),
		"unique_probes_tenant_id_name_key", nameTaken)
	for id, state := range map[string]uniqueProbe{
		"U-3": {Name: "gamma", Code: "C", Label: "green", RefA: "C", RefB: "green"},
		"U-4": {Name: "delta", Code: "D", Label: "purple", RefA: "D", RefB: "purple"},
	} {
		if err := repo.Save(ctx, id, state, 0); err != nil {
			t.Fatalf("Save(%s) = %v", id, err)
		}
	}
	if err := nested.Commit(ctx); err != nil {
		t.Fatalf("nested Commit() = %v, want pgx's own savepoint still there to release", err)
	}
	if err := outer.Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v", err)
	}
	for _, id := range []string{"U-3", "U-4"} {
		if _, _, err := uniqueTable.Reader(postgres.NewReadPool(pool)).Load(ctx, id); err != nil {
			t.Fatalf("Load(%s) = %v, want the Saves that followed the violation committed", id, err)
		}
	}
}

func TestACancelledSaveUnderTheSavepointFailsAndLeavesNothing(t *testing.T) {
	pool := openPool(t)
	ctx := scopedTo(t, "acme")
	cancelled, cancel := context.WithCancel(ctx)
	uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) ports.Repository[string, uniqueProbe] {
		return uniqueTable.Repository(tx)
	})

	err := uow.Within(cancelled, func(ctx context.Context, repo ports.Repository[string, uniqueProbe]) error {
		cancel()
		return repo.Save(ctx, "U-1", uniqueProbe{Name: "alpha", Code: "A", Label: "red", RefA: "A", RefB: "red"}, 0)
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Within() = %v, want context.Canceled", err)
	}
	if _, _, err := uniqueTable.Reader(postgres.NewReadPool(pool)).Load(ctx, "U-1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound: the cancelled Save rolled back", err)
	}
	if err := saveUnique(t, ctx, pool, "U-1", uniqueProbe{Name: "alpha", Code: "A", Label: "red", RefA: "A", RefB: "red"}, 0); err != nil {
		t.Fatalf("Save() after the cancellation = %v, want the pool still usable", err)
	}
}

func statementNames(spans []tracetest.SpanStub) []string {
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.Name)
	}
	return names
}

func oneConn(cfg *pgxpool.Config) { cfg.MaxConns = 1 }

func execMode(mode pgx.QueryExecMode) func(*pgxpool.Config) {
	return func(cfg *pgxpool.Config) { cfg.ConnConfig.DefaultQueryExecMode = mode }
}

var execModes = []struct {
	name  string
	mode  pgx.QueryExecMode
	trips int64
}{
	{"cache statement", pgx.QueryExecModeCacheStatement, 1},
	{"cache describe", pgx.QueryExecModeCacheDescribe, 2},
	{"describe exec", pgx.QueryExecModeDescribeExec, 3},
	{"exec", pgx.QueryExecModeExec, 1},
	{"simple protocol", pgx.QueryExecModeSimpleProtocol, 1},
}

func TestEveryQueryExecModeSavesUnderTheSavepoint(t *testing.T) {
	for _, tt := range execModes {
		t.Run(tt.name, func(t *testing.T) {
			pool, writes := wireCountedPool(t, execMode(tt.mode))
			ctx := scopedTo(t, "acme")
			seedUnique(t, ctx, pool)

			var trips int64
			for n := range 3 {
				conn, err := pool.Begin(ctx)
				if err != nil {
					t.Fatalf("Begin() = %v", err)
				}
				before := writes.Load()
				v := fmt.Sprint("w-", n)
				err = uniqueTable.Repository(postgres.NewTx(conn)).Save(ctx, v, uniqueProbe{Name: v, Code: v, Label: v, RefA: v, RefB: v}, 0)
				trips = writes.Load() - before
				_ = conn.Rollback(ctx)
				if err != nil {
					t.Fatalf("Save() = %v", err)
				}
			}
			if trips != tt.trips {
				t.Fatalf("round trips of a Save on a warm connection = %d, want %d", trips, tt.trips)
			}

			tx := begin(t, pool)
			requireViolation(t, uniqueTable.Repository(tx).Save(ctx, "U-3", uniqueProbe{Name: "alpha", Code: "C", Label: "green", RefA: "C", RefB: "green"}, 0),
				"unique_probes_tenant_id_name_key", nameTaken)
			if err := uniqueTable.Repository(tx).Save(ctx, "U-3", uniqueProbe{Name: "gamma", Code: "C", Label: "green", RefA: "C", RefB: "green"}, 0); err != nil {
				t.Fatalf("Save() after the violation = %v, want the transaction usable", err)
			}
		})
	}
}

// driftedTable writes a column the schema lacks, which the server refuses while
// it parses the statement, before any row is touched.
var driftedTable = func() postgres.Table[string, uniqueProbe] {
	drifted := uniqueTable
	drifted.Columns = []string{"name", "code", "label", "ref_a", "ref_c"}
	return drifted
}()

func TestAWriteTheServerCannotPrepareIsUndoneByTheSavepoint(t *testing.T) {
	for _, tt := range execModes {
		t.Run(tt.name, func(t *testing.T) {
			pool := openPoolWith(t, execMode(tt.mode))
			ctx := scopedTo(t, "acme")
			tx := begin(t, pool)

			for range 2 {
				err := driftedTable.Repository(tx).Save(ctx, "U-9", uniqueProbe{Name: "n", Code: "c", Label: "l", RefA: "a", RefB: "b"}, 0)
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "42703" {
					t.Fatalf("Save() over a drifted column = %v, want the PgError with SQLSTATE 42703", err)
				}
				if msg := err.Error(); strings.Contains(msg, "savepoint") || strings.Contains(msg, "preprocessing") {
					t.Fatalf("Save() = %q, want the failure of the write, not of the savepoint nor of the batch", msg)
				}
			}
			if err := uniqueTable.Repository(tx).Save(ctx, "U-1", uniqueProbe{Name: "alpha", Code: "A", Label: "red", RefA: "A", RefB: "red"}, 0); err != nil {
				t.Fatalf("Save() after the refused statement = %v, want ROLLBACK TO to have left the transaction usable", err)
			}
		})
	}
}

func holdName(t *testing.T, name string) func() {
	t.Helper()
	holder, err := openAdminPool(t).Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	if _, err := holder.Exec(context.Background(), "INSERT INTO unique_probes VALUES ('acme', 'H-1', 1, $1, 'H', 'H', 'H', 'H')", name); err != nil {
		t.Fatalf("INSERT = %v", err)
	}
	return func() { _ = holder.Rollback(context.Background()) }
}

func TestTheStatementsOfASavepointBatchShareTheWaitOnTheIndex(t *testing.T) {
	const held = 250 * time.Millisecond
	pool, tracer, spans := tracedPool(t, oneConn)
	ctx := scopedTo(t, "acme")
	if err := saveUnique(t, ctx, pool, "W-1", uniqueProbe{Name: "w", Code: "w", Label: "w", RefA: "w", RefB: "w"}, 0); err != nil {
		t.Fatalf("Save() = %v", err)
	}
	release := holdName(t, "held")
	time.AfterFunc(held, release)

	conn, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	spanCtx, parent := tracer.Start(ctx, "save")
	err = uniqueTable.Repository(postgres.NewTx(conn)).Save(spanCtx, "B-1", uniqueProbe{Name: "held", Code: "X", Label: "X", RefA: "X", RefB: "X"}, 0)
	parent.End()
	_ = conn.Rollback(context.Background())
	if err != nil {
		t.Fatalf("Save() once the holder rolled back = %v", err)
	}

	got := statementsOf(spans, parent)
	if names := statementNames(got); !slices.Equal(names, []string{"SAVEPOINT", "INSERT unique_probes", "RELEASE SAVEPOINT"}) {
		t.Fatalf("statements = %v", names)
	}
	for _, span := range got {
		if !span.StartTime.Equal(got[0].StartTime) {
			t.Fatalf("span %s starts at %v, want the send of the batch at %v", span.Name, span.StartTime, got[0].StartTime)
		}
	}
	if took := got[1].EndTime.Sub(got[1].StartTime); took < held*4/5 {
		t.Fatalf("INSERT took %v, want the wait on the unique index, about %v", took, held)
	}
}

func TestASaveThatTimesOutUnderTheSavepointIsNotTheSavepointFailing(t *testing.T) {
	pool, tracer, spans := tracedPool(t, oneConn)
	ctx := scopedTo(t, "acme")
	if err := saveUnique(t, ctx, pool, "W-1", uniqueProbe{Name: "w", Code: "w", Label: "w", RefA: "w", RefB: "w"}, 0); err != nil {
		t.Fatalf("Save() = %v", err)
	}
	release := holdName(t, "held")
	defer release()

	conn, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	timed, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	spanCtx, parent := tracer.Start(timed, "save")
	err = uniqueTable.Repository(postgres.NewTx(conn)).Save(spanCtx, "B-1", uniqueProbe{Name: "held", Code: "X", Label: "X", RefA: "X", RefB: "X"}, 0)
	parent.End()
	_ = conn.Rollback(context.Background())

	if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ports.ErrUniqueViolation) || strings.Contains(err.Error(), "savepoint") {
		t.Fatalf("Save() = %v, want the deadline, neither a violation nor a failure of the savepoint", err)
	}
	got := statementsOf(spans, parent)
	if names := statementNames(got); !slices.Equal(names, []string{"SAVEPOINT", "INSERT unique_probes", "RELEASE SAVEPOINT"}) {
		t.Fatalf("statements = %v, want the batch alone: its connection is gone; nothing is left to undo", names)
	}
	for _, span := range got {
		if span.Status.Code != codes.Error {
			t.Fatalf("span %s status = %+v, want every statement in flight failed", span.Name, span.Status)
		}
	}
	release()
	if err := saveUnique(t, ctx, pool, "P-1", uniqueProbe{Name: "p", Code: "p", Label: "p", RefA: "p", RefB: "p"}, 0); err != nil {
		t.Fatalf("Save() after the timeout = %v, want the pool usable", err)
	}
}

func TestASavepointSentAloneThatFailsBeforeItsResultIsNotTheSavepointRefused(t *testing.T) {
	tests := []struct {
		name   string
		breaks func(ctx context.Context, tx *postgres.Tx) context.Context
		want   string
	}{
		{"a cancelled caller", func(ctx context.Context, _ *postgres.Tx) context.Context {
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			return cancelled
		}, "context canceled"},
		{"a closed connection", func(ctx context.Context, tx *postgres.Tx) context.Context {
			if err := postgres.ConnOf(tx).Conn().Close(context.Background()); err != nil {
				t.Fatalf("Close() = %v", err)
			}
			return ctx
		}, "conn closed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, tracer, spans := tracedPool(t, oneConn)
			ctx := scopedTo(t, "acme")
			tx := begin(t, pool)
			spanCtx, parent := tracer.Start(tt.breaks(ctx, tx), "save")

			err := uniqueTable.Repository(tx).Save(spanCtx, "C-1", uniqueProbe{Name: "c", Code: "c", Label: "c", RefA: "c", RefB: "c"}, 0)
			parent.End()

			if err == nil || !strings.Contains(err.Error(), tt.want) || strings.Contains(err.Error(), "savepoint") {
				t.Fatalf("Save() = %v, want %q without the label of a SAVEPOINT the server refused", err, tt.want)
			}
			if names := statementNames(statementsOf(spans, parent)); !slices.Equal(names, []string{"SAVEPOINT"}) {
				t.Fatalf("statements = %v, want the SAVEPOINT alone: nothing says it was established", names)
			}
		})
	}
}

func preparedOn(t *testing.T, tx *postgres.Tx) int {
	t.Helper()
	var n int
	if err := postgres.ConnOf(tx).QueryRow(context.Background(), "SELECT count(*) FROM pg_prepared_statements").Scan(&n); err != nil {
		t.Fatalf("count prepared statements = %v", err)
	}
	return n
}

func TestEverySaveReusesTheOneSavepointName(t *testing.T) {
	pool := openPoolWith(t, oneConn)
	ctx := scopedTo(t, "acme")
	seedUnique(t, ctx, pool)
	conn, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	t.Cleanup(func() { _ = conn.Rollback(context.Background()) })
	tx := postgres.NewTx(conn)
	repo := uniqueTable.Repository(tx)
	save := func(id string) {
		t.Helper()
		if err := repo.Save(ctx, id, uniqueProbe{Name: id, Code: id, Label: id, RefA: id, RefB: id}, 0); err != nil {
			t.Fatalf("Save(%s) = %v", id, err)
		}
	}

	requireViolation(t, repo.Save(ctx, "U-5", uniqueProbe{Name: "alpha", Code: "E", Label: "e", RefA: "e", RefB: "e"}, 0), "unique_probes_tenant_id_name_key", nameTaken)
	save("S-1")
	requireViolation(t, repo.Save(ctx, "U-6", uniqueProbe{Name: "S-1", Code: "F", Label: "f", RefA: "f", RefB: "f"}, 0), "unique_probes_tenant_id_name_key", nil)
	nested, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("nested Begin() = %v", err)
	}
	save("S-2")
	if err := nested.Rollback(ctx); err != nil {
		t.Fatalf("nested Rollback() = %v", err)
	}
	save("S-3")
	prepared := preparedOn(t, tx)
	for n := range 600 {
		save(fmt.Sprint("M-", n))
	}
	if got := preparedOn(t, tx); got != prepared {
		t.Fatalf("prepared statements after 600 Saves = %d, want the %d before them: one savepoint name, one statement", got, prepared)
	}
	if err := conn.Commit(ctx); err != nil {
		t.Fatalf("Commit() = %v", err)
	}

	reader := uniqueTable.Reader(postgres.NewReadPool(pool))
	for id, want := range map[string]bool{"S-1": true, "S-2": false, "S-3": true, "M-599": true, "U-5": false, "U-6": false} {
		if _, _, err := reader.Load(ctx, id); (err == nil) != want {
			t.Errorf("Load(%s) = %v, want present %v: each ROLLBACK TO and RELEASE reached the newest savepoint of the name", id, err, want)
		}
	}
}
