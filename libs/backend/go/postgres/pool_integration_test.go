//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const insertPending = `
INSERT INTO dmpf_outbox (
	message_id, message_type, schema_version,
	aggregate_type, aggregate_id, aggregate_version,
	partition_key, destination,
	payload, payload_hash, metadata,
	occurred_at, available_at, attempt_count, status
) VALUES ($1, 'com.company.orders.order-placed.v1', 'type.googleapis.com/company.orders.event.v1.OrderPlaced',
	'order', 'o-1', 1,
	'pk-1', $2,
	'\x0a', 'sha-256:stub', '{}'::jsonb,
	1, 1, 0, $3)`

func enqueue(t *testing.T, pool *pgxpool.Pool, messageID, destination, status string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), insertPending, messageID, destination, status); err != nil {
		t.Fatalf("enqueue(%s) = %v, want nil", messageID, err)
	}
}

func TestAnEmptyOutboxBelongsToWhoeverAsks(t *testing.T) {
	pool := openPool(t)

	if err := postgres.AssertOwnOutbox(context.Background(), pool, []string{"orders.events"}); err != nil {
		t.Fatalf("AssertOwnOutbox() = %v, want nil", err)
	}
}

func TestAnOutboxHoldingOnlyOwnDestinationsPasses(t *testing.T) {
	pool := openPool(t)
	enqueue(t, pool, "m-own-1", "orders.events", "pending")
	enqueue(t, pool, "m-own-2", "orders.audit", "publishing")

	err := postgres.AssertOwnOutbox(context.Background(), pool, []string{"orders.events", "orders.audit"})

	if err != nil {
		t.Fatalf("AssertOwnOutbox() = %v, want nil", err)
	}
}

func TestAForeignDestinationIsRefusedAndNamed(t *testing.T) {
	pool := openPool(t)
	enqueue(t, pool, "m-foreign-1", "reservations.events", "pending")

	err := postgres.AssertOwnOutbox(context.Background(), pool, []string{"orders.events"})

	if err == nil {
		t.Fatal("AssertOwnOutbox() = nil; the relay drains without filtering by destination and would carry away another context's record")
	}
	if !strings.Contains(err.Error(), "reservations.events") {
		t.Fatalf("AssertOwnOutbox() = %v, want the message to name the foreign destination", err)
	}
}

func TestASettledRecordDoesNotClaimTheDatabase(t *testing.T) {
	pool := openPool(t)
	enqueue(t, pool, "m-settled-1", "reservations.events", "published")

	err := postgres.AssertOwnOutbox(context.Background(), pool, []string{"orders.events"})

	if err != nil {
		t.Fatalf("AssertOwnOutbox() = %v, want nil: only pending and publishing records are still the relay's to drain", err)
	}
}
