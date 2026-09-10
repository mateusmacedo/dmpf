//go:build integration

package orderspg_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	orderspg "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres/example/orders"
)

// sqlRecorder is a pgx.QueryTracer that keeps every statement the connection
// sent, begin and commit included: it is the only witness that a read ran
// outside a transaction, because the pool's counters cannot tell the two apart.
type sqlRecorder struct {
	mu   sync.Mutex
	sent []string
}

func (r *sqlRecorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, data.SQL)
	return ctx
}

func (r *sqlRecorder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (r *sqlRecorder) statements() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.sent...)
}

// tracedPool is a second pool over the same database whose connections report
// to the recorder; the fixture pool keeps handling seeding and truncation.
func tracedPool(t *testing.T, pool *pgxpool.Pool) (*pgxpool.Pool, *sqlRecorder) {
	t.Helper()

	recorder := &sqlRecorder{}
	cfg := pool.Config()
	cfg.ConnConfig.Tracer = recorder
	traced, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() = %v, want nil", err)
	}
	t.Cleanup(traced.Close)
	return traced, recorder
}

func TestNewReaderLoadsWhatTheRepositoryWrote(t *testing.T) {
	pool := openPool(t)
	seed(t, pool)

	loaded, version, err := orderspg.NewReader(pool).Load(context.Background(), repoOrderID)

	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
	if !loaded.Equal(snapshot(1)) {
		t.Fatalf("snapshot = %+v, want %+v", loaded, snapshot(1))
	}
}

func TestNewReaderReportsAnAbsentOrder(t *testing.T) {
	pool := openPool(t)

	_, _, err := orderspg.NewReader(pool).Load(context.Background(), "o-absent")

	if !errors.Is(err, dmpfports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound", err)
	}
}

// UOW-11: a query is given read access without the write side, and that read
// must not open a transaction — one SELECT on the pool, no begin, no commit.
func TestNewReaderNeverOpensATransaction(t *testing.T) {
	pool := openPool(t)
	seed(t, pool)
	traced, recorder := tracedPool(t, pool)

	if _, _, err := orderspg.NewReader(traced).Load(context.Background(), repoOrderID); err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}

	sent := recorder.statements()
	if len(sent) != 1 {
		t.Fatalf("the read sent %d statements %q, want exactly the SELECT", len(sent), sent)
	}
	for _, statement := range sent {
		switch strings.ToLower(strings.TrimSpace(statement)) {
		case "begin", "commit", "rollback":
			t.Fatalf("the read sent %q: a Reader must not open a transaction (UOW-11)", statement)
		}
	}
	if !strings.HasPrefix(strings.TrimSpace(sent[0]), "SELECT") {
		t.Fatalf("statement = %q, want the SELECT of the aggregate", sent[0])
	}
}
