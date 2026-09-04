//go:build integration

package dmpfpostgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
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

	purge, err := dmpfpostgres.PurgePublished(context.Background(), pool, dmpfports.Instant(200))
	if err != nil {
		t.Fatalf("PurgePublished() = %v, want nil", err)
	}

	if purge.Count != 1 {
		t.Errorf("Purge.Count = %d, want 1 (OBX-17)", purge.Count)
	}
	if purge.Before != dmpfports.Instant(200) {
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

	purge, err := dmpfpostgres.PurgePublished(context.Background(), pool, dmpfports.Instant(200))
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

func survived(t *testing.T, pool *pgxpool.Pool, id string) bool {
	t.Helper()

	var found bool
	if err := pool.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM dmpf_outbox WHERE message_id = $1)", id).Scan(&found); err != nil {
		t.Fatalf("EXISTS = %v, want nil", err)
	}
	return found
}
