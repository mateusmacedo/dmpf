//go:build integration

package app_test

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

func purgePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return pg.OpenPool(t, pg.Options{Project: "app", Capabilities: []postgres.Capability{postgres.Outbox, postgres.Inbox}})
}

func seed(t *testing.T, pool *pgxpool.Pool, statement string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), statement, args...); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func remaining(t *testing.T, pool *pgxpool.Pool, query string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	slices.Sort(ids)
	return ids
}

const seedOutbox = `
INSERT INTO outbox (
  message_id, message_type, schema_version, aggregate_type, aggregate_id,
  aggregate_version, destination, payload, payload_hash,
  occurred_at, available_at, status, published_at
) VALUES ($1, 'com.company.orders.order-placed.v1', 'type.googleapis.com/x',
          'orders.Order', 'o-1', 1, 'orders.events', '\x0a03612d31', 'hash',
          100, 100, $2, $3)`

const seedInbox = `
INSERT INTO inbox (consumer_name, message_id, message_type, payload_hash, received_at, processed_at, status, outcome, expires_at)
VALUES ($1, $2, 'example', 'h1', 1, $3, 'processed', $4, $5)`

func TestPurgeOutboxRemovesThePublishedRowsBeforeTheCutoff(t *testing.T) {
	pool := purgePool(t)
	seed(t, pool, seedOutbox, "m-published-old", "published", int64(100))
	seed(t, pool, seedOutbox, "m-published-new", "published", int64(300))
	seed(t, pool, seedOutbox, "m-pending", "pending", nil)

	removed, err := app.PurgeOutbox(pool)(context.Background(), 200, 10)

	if err != nil || removed != 1 {
		t.Fatalf("PurgeOutbox = (%d, %v), want (1, nil)", removed, err)
	}
	if got := remaining(t, pool, "SELECT message_id FROM outbox"); !slices.Equal(got, []string{"m-pending", "m-published-new"}) {
		t.Fatalf("outbox kept %v", got)
	}
}

func TestPurgeCommandInboxRemovesOnlyTheExpiredCommandsOfTheConsumer(t *testing.T) {
	pool := purgePool(t)
	seed(t, pool, seedInbox, "orders.commands", "k-expired", int64(1), []byte{1}, int64(100))
	seed(t, pool, seedInbox, "orders.commands", "k-live", int64(1), []byte{1}, int64(1_000))
	seed(t, pool, seedInbox, "bookings.commands", "k-expired", int64(1), []byte{1}, int64(100))

	removed, err := app.PurgeCommandInbox(pool, "orders.commands")(context.Background(), 500, 10)

	if err != nil || removed != 1 {
		t.Fatalf("PurgeCommandInbox = (%d, %v), want (1, nil)", removed, err)
	}
	want := []string{"bookings.commands|k-expired", "orders.commands|k-live"}
	if got := remaining(t, pool, "SELECT consumer_name || '|' || message_id FROM inbox"); !slices.Equal(got, want) {
		t.Fatalf("inbox kept %v, want %v", got, want)
	}
}

func TestPurgeMessageInboxRemovesOnlyTheMessagesTheConsumerProcessedBeforeTheCutoff(t *testing.T) {
	pool := purgePool(t)
	seed(t, pool, seedInbox, "reservations", "m-old", int64(100), nil, nil)
	seed(t, pool, seedInbox, "reservations", "m-new", int64(300), nil, nil)
	seed(t, pool, seedInbox, "orders", "m-old", int64(100), nil, nil)

	removed, err := app.PurgeMessageInbox(pool, "reservations")(context.Background(), 200, 10)

	if err != nil || removed != 1 {
		t.Fatalf("PurgeMessageInbox = (%d, %v), want (1, nil)", removed, err)
	}
	want := []string{"orders|m-old", "reservations|m-new"}
	if got := remaining(t, pool, "SELECT consumer_name || '|' || message_id FROM inbox"); !slices.Equal(got, want) {
		t.Fatalf("inbox kept %v, want %v", got, want)
	}
}
