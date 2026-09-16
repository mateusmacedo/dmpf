//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// seedOutboxRow writes drain states Enqueue never produces, which is the only
// way to reach a purge: a row is born pending and only the relay moves it.
func seedOutboxRow(t *testing.T, pool *pgxpool.Pool, id, status string, publishedAt *int64) {
	t.Helper()

	_, err := pool.Exec(context.Background(), `
		INSERT INTO dmpf_outbox (
			message_id, message_type, schema_version, aggregate_type, aggregate_id,
			aggregate_version, destination, payload, payload_hash,
			occurred_at, available_at, status, published_at
		) VALUES ($1, 'com.company.orders.order-placed.v1', 'type.googleapis.com/x',
		          'orders.Order', 'o-1', 1, 'orders.events', '\x0a03612d31', 'hash',
		          100, 100, $2, $3)`, id, status, publishedAt)
	if err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func TestPurgePublishedRemovesOnlyPublishedRowsBefore(t *testing.T) {
	pool := openPool(t)
	at100, at300 := int64(100), int64(300)

	seedOutboxRow(t, pool, "m-pending", "pending", nil)
	seedOutboxRow(t, pool, "m-publishing", "publishing", nil)
	seedOutboxRow(t, pool, "m-published-100", "published", &at100)
	seedOutboxRow(t, pool, "m-published-300", "published", &at300)
	seedOutboxRow(t, pool, "m-failed", "failed", nil)

	purge, err := postgres.PurgePublished(context.Background(), pool, ports.Instant(200))
	if err != nil {
		t.Fatalf("PurgePublished() = %v, want nil", err)
	}

	if purge.Count != 1 {
		t.Errorf("Purge.Count = %d, want 1 (OBX-17)", purge.Count)
	}
	if purge.Before != ports.Instant(200) {
		t.Errorf("Purge.Before = %d, want 200 — the evidence says up to when", purge.Before)
	}
	if got := enqueued(t, pool); got != 4 {
		t.Errorf("kept %d rows, want 4", got)
	}
	if survived(t, pool, "m-published-100") {
		t.Error("m-published-100 survived, but it was published before the cutoff")
	}
	for _, id := range []string{"m-pending", "m-publishing", "m-published-300", "m-failed"} {
		if !survived(t, pool, id) {
			t.Errorf("%s was purged, but only published rows before the cutoff may go", id)
		}
	}
}

func TestPurgePublishedReportsAnEmptyPurge(t *testing.T) {
	pool := openPool(t)
	seedOutboxRow(t, pool, "m-pending", "pending", nil)

	purge, err := postgres.PurgePublished(context.Background(), pool, ports.Instant(200))
	if err != nil {
		t.Fatalf("PurgePublished() = %v, want nil", err)
	}

	if purge.Count != 0 {
		t.Errorf("Purge.Count = %d, want 0", purge.Count)
	}
	if got := enqueued(t, pool); got != 1 {
		t.Errorf("kept %d rows, want 1", got)
	}
}

func seedInboxRow(t *testing.T, pool *pgxpool.Pool, consumer, id, status string, processedAt int64) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO dmpf_inbox (consumer_name, message_id, message_type, payload_hash, received_at, processed_at, status)
		VALUES ($1, $2, 'example', 'h1', 100, $3, $4)`, consumer, id, processedAt, status)
	if err != nil {
		t.Fatalf("seedInboxRow %s/%s: %v", consumer, id, err)
	}
}

func inboxCount(t *testing.T, pool *pgxpool.Pool, consumer string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM dmpf_inbox WHERE consumer_name = $1`, consumer).Scan(&count); err != nil {
		t.Fatalf("count inbox = %v", err)
	}
	return count
}

func TestPurgeInboxRemovesOnlyScopedRowsBelowCutoff(t *testing.T) {
	pool := openPool(t)

	seedInboxRow(t, pool, "orders", "m-1", "processed", 100)
	seedInboxRow(t, pool, "orders", "m-2", "rejected", 200)
	seedInboxRow(t, pool, "orders", "m-3", "processed", 400)
	seedInboxRow(t, pool, "billing", "m-1", "processed", 100)

	purge, err := postgres.PurgeInbox(context.Background(), pool, "orders", ports.Instant(300))
	if err != nil {
		t.Fatalf("PurgeInbox() = %v, want nil", err)
	}

	if purge.Consumer != "orders" {
		t.Errorf("Consumer = %q, want %q", purge.Consumer, "orders")
	}
	if purge.Removed != 2 {
		t.Errorf("Removed = %d, want 2", purge.Removed)
	}
	if purge.Before != ports.Instant(300) {
		t.Errorf("Before = %d, want 300", purge.Before)
	}
	if got := inboxCount(t, pool, "orders"); got != 1 {
		t.Errorf("orders remaining = %d, want 1", got)
	}
	if got := inboxCount(t, pool, "billing"); got != 1 {
		t.Errorf("billing remaining = %d, want 1 — billing must be untouched", got)
	}
}

func TestPurgeInboxEmptyPurge(t *testing.T) {
	pool := openPool(t)
	seedInboxRow(t, pool, "orders", "m-1", "processed", 500)

	purge, err := postgres.PurgeInbox(context.Background(), pool, "orders", ports.Instant(100))
	if err != nil {
		t.Fatalf("PurgeInbox() = %v, want nil", err)
	}
	if purge.Removed != 0 {
		t.Errorf("Removed = %d, want 0", purge.Removed)
	}
}

func survived(t *testing.T, pool *pgxpool.Pool, id string) bool {
	t.Helper()

	var found bool
	if err := pool.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM dmpf_outbox WHERE message_id = $1)", id).Scan(&found); err != nil {
		t.Fatalf("EXISTS = %v, want nil", err)
	}
	return found
}
