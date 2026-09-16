//go:build integration

package reservationspg_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	reservationspg "github.com/mateusmacedo/dmpf/libs/backend/go/postgres/example/reservations"
)

// sqlRecorder keeps every statement the connection sent, begin and commit
// included: the only witness that a read ran outside a transaction.
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
	want, wantVersion := load(t, pool)

	got, version, err := reservationspg.NewReader(pool).Load(context.Background(), repoOrderID)

	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if version != wantVersion || !got.Equal(want) {
		t.Fatalf("Load() = %+v v%d, want %+v v%d", got, version, want, wantVersion)
	}
}

func TestNewReaderReportsAnAbsentReservation(t *testing.T) {
	pool := openPool(t)

	_, _, err := reservationspg.NewReader(pool).Load(context.Background(), "o-absent")

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound", err)
	}
}

// UOW-11: one SELECT on the pool, no begin, no commit.
func TestNewReaderNeverOpensATransaction(t *testing.T) {
	pool := openPool(t)
	seed(t, pool)
	traced, recorder := tracedPool(t, pool)

	if _, _, err := reservationspg.NewReader(traced).Load(context.Background(), repoOrderID); err != nil {
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
