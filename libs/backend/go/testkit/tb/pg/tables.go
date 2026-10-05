package pg

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// readTimeout bounds each read, so a Postgres that stops answering fails the
// test by name instead of letting it hang until go test's -timeout.
const readTimeout = 10 * time.Second

// Enqueued is one outbox record as the drain will read it, the effect edge of
// a producing context.
type Enqueued struct {
	MessageID        string
	MessageType      string
	SchemaVersion    string
	AggregateVersion int64
	Destination      string
	Status           string
}

// Outbox reads what the use cases left for the relay, in the order it was
// enqueued, so a test asserts the sequence and not only the presence.
func Outbox(t testing.TB, pool *pgxpool.Pool) []Enqueued {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	rows, err := pool.Query(ctx, `SELECT message_id, message_type, schema_version, aggregate_version, destination, status
		FROM outbox ORDER BY id`)
	if err != nil {
		t.Fatalf("pg.Outbox: %v", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Enqueued, error) {
		var e Enqueued
		err := row.Scan(&e.MessageID, &e.MessageType, &e.SchemaVersion, &e.AggregateVersion, &e.Destination, &e.Status)
		return e, err
	})
	if err != nil {
		t.Fatalf("pg.Outbox: %v", err)
	}
	return out
}

func Settled(t testing.TB, pool *pgxpool.Pool, want int, timeout time.Duration) map[string]string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	settled := map[string]string{}
	for remaining := timeout; remaining > 0; remaining = time.Until(deadline) {
		settled = published(t, pool, min(readTimeout, remaining))
		if len(settled) >= want {
			return settled
		}
		time.Sleep(min(100*time.Millisecond, time.Until(deadline)))
	}
	t.Fatalf("pg.Settled: %d of %d records settled within %v", len(settled), want, timeout)
	return nil
}

func published(t testing.TB, pool *pgxpool.Pool, within time.Duration) map[string]string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	rows, err := pool.Query(ctx, `SELECT message_id, payload_hash FROM outbox WHERE status = 'published'`)
	if err != nil {
		t.Fatalf("pg.Settled: %v", err)
	}
	defer rows.Close()
	settled := map[string]string{}
	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			t.Fatalf("pg.Settled: scan: %v", err)
		}
		settled[id] = hash
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("pg.Settled: %v", err)
	}
	return settled
}

// Counts reads how many rows each table holds in a single statement, so the
// numbers come from one snapshot even while another process writes.
func Counts(t testing.TB, pool *pgxpool.Pool, tables ...string) map[string]int64 {
	t.Helper()
	if len(tables) == 0 {
		t.Fatalf("pg.Counts: name at least one table")
		return nil
	}
	columns := make([]string, len(tables))
	for i, table := range tables {
		columns[i] = "(SELECT count(*) FROM " + pgx.Identifier{table}.Sanitize() + ")"
	}
	counts := make([]int64, len(tables))
	targets := make([]any, len(tables))
	for i := range counts {
		targets[i] = &counts[i]
	}
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	if err := pool.QueryRow(ctx, "SELECT "+strings.Join(columns, ", ")).Scan(targets...); err != nil {
		t.Fatalf("pg.Counts: %v", err)
	}
	out := make(map[string]int64, len(tables))
	for i, table := range tables {
		out[table] = counts[i]
	}
	return out
}

func Tables(t testing.TB, pool *pgxpool.Pool) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	rows, err := pool.Query(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name")
	if err != nil {
		t.Fatalf("pg.Tables: %v", err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("pg.Tables: %v", err)
	}
	return names
}
