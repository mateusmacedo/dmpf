//go:build integration

package pg

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func readerPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return OpenPool(t, Options{Project: "pgkit_readers", Capabilities: []postgres.Capability{postgres.Outbox, postgres.Inbox}})
}

func enqueue(t *testing.T, pool *pgxpool.Pool, id, status string, version int64) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO outbox (
			message_id, message_type, schema_version, aggregate_type, aggregate_id,
			aggregate_version, destination, payload, payload_hash,
			occurred_at, available_at, status
		) VALUES ($1, 'com.company.orders.order-placed.v1', 'type.googleapis.com/x',
		          'orders.Order', 'o-1', $3, 'orders.events', '\x0a03612d31', 'hash-'||$1,
		          100, 100, $2)`, id, status, version)
	if err != nil {
		t.Fatalf("enqueue %s: %v", id, err)
	}
}

func TestTablesListsThePublicTablesByName(t *testing.T) {
	pool := readerPool(t)

	if got, want := Tables(t, pool), []string{"inbox", "outbox", "quarantine"}; !slices.Equal(got, want) {
		t.Fatalf("Tables() = %v, want %v", got, want)
	}
}

func TestOutboxReadsTheRecordsInTheOrderTheyWereEnqueued(t *testing.T) {
	pool := readerPool(t)
	enqueue(t, pool, "m-2", "pending", 2)
	enqueue(t, pool, "m-1", "published", 1)

	got := Outbox(t, pool)

	want := []Enqueued{
		{MessageID: "m-2", MessageType: "com.company.orders.order-placed.v1", SchemaVersion: "type.googleapis.com/x", AggregateVersion: 2, Destination: "orders.events", Status: "pending"},
		{MessageID: "m-1", MessageType: "com.company.orders.order-placed.v1", SchemaVersion: "type.googleapis.com/x", AggregateVersion: 1, Destination: "orders.events", Status: "published"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Outbox() = %+v, want %+v", got, want)
	}
}

func TestSettledAnswersThePublishedRecordsWithTheirHash(t *testing.T) {
	pool := readerPool(t)
	enqueue(t, pool, "m-1", "published", 1)
	enqueue(t, pool, "m-2", "pending", 2)

	got := Settled(t, pool, 1, time.Second)

	if len(got) != 1 || got["m-1"] != "hash-m-1" {
		t.Fatalf("Settled() = %v, want only m-1 with its hash", got)
	}
}

func TestCountsReadsEveryTableItIsGiven(t *testing.T) {
	pool := readerPool(t)
	enqueue(t, pool, "m-1", "pending", 1)
	enqueue(t, pool, "m-2", "pending", 2)

	got := Counts(t, pool, "outbox", "inbox")

	if len(got) != 2 || got["outbox"] != 2 || got["inbox"] != 0 {
		t.Fatalf("Counts() = %v, want outbox 2 and inbox 0", got)
	}
}

type fatalRecorder struct {
	testing.TB
	fatal string
}

func (f *fatalRecorder) Helper() {}

func (f *fatalRecorder) Fatalf(format string, args ...any) { f.fatal = fmt.Sprintf(format, args...) }

func TestCountsRefusesACallThatNamesNoTable(t *testing.T) {
	r := &fatalRecorder{TB: t}

	if got := Counts(r, nil); got != nil || !strings.Contains(r.fatal, "at least one table") {
		t.Fatalf("Counts() = %v with fatal %q, want a refusal naming the missing table", got, r.fatal)
	}
}
