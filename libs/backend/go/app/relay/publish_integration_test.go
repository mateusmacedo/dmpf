//go:build integration

package relay

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// Duplicated from the provider's harness on purpose: a _test.go file is never
// importable, so this package cannot reuse another package's openPool. An
// exported test kit is KRN-11's.
func openSingleConnPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DMPF_PG_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("DMPF_PG_DSN is empty in CI: integration tests must not skip silently")
		}
		t.Skip("DMPF_PG_DSN not set; run `docker compose -f infra/local/docker-compose.yml --profile postgres up -d` and export DMPF_PG_DSN=postgres://app:app@localhost:5432/app?sslmode=disable")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig() = %v, want nil", err)
	}
	// One connection is what makes the proof possible: a connection the relay
	// forgot to release is the only connection, and the probe cannot get it.
	config.MaxConns = 1

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("NewWithConfig() = %v, want nil", err)
	}
	t.Cleanup(pool.Close)

	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate() = %v, want nil", err)
	}
	truncateOutbox(t, pool)
	t.Cleanup(func() { truncateOutbox(t, pool) })

	return pool
}

func truncateOutbox(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	const stmt = "TRUNCATE outbox, inbox, quarantine, dmpf_example_orders, dmpf_example_reservations"
	if _, err := pool.Exec(context.Background(), stmt); err != nil {
		t.Fatalf("TRUNCATE = %v, want nil", err)
	}
}

type systemClock struct{}

func (systemClock) Now() ports.Instant { return ports.Instant(time.Now().UnixNano()) }

// blockingPublisher stops inside Publish so the test can inspect the pool while
// the transport I/O is in flight.
type blockingPublisher struct {
	entered chan struct{}
	release chan struct{}
}

func newBlockingPublisher() *blockingPublisher {
	return &blockingPublisher{entered: make(chan struct{}), release: make(chan struct{})}
}

func (p *blockingPublisher) Publish(context.Context, string, []byte) error {
	close(p.entered)
	<-p.release
	return nil
}

// probeCanAcquire reports whether a caller can still get a connection out of
// the pool within the deadline.
func probeCanAcquire(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return false
	}
	defer conn.Release()

	var one int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return false
	}
	return one == 1
}

func seedOutboxRow(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	record := publishableRecord(t)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO outbox (
			message_id, message_type, schema_version,
			aggregate_type, aggregate_id, aggregate_version,
			partition_key, destination,
			payload, payload_hash, metadata,
			occurred_at, available_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		record.MessageID, record.MessageType, record.SchemaVersion,
		record.AggregateType, record.AggregateID, record.AggregateVersion,
		record.PartitionKey, record.Destination,
		record.Payload, record.PayloadHash, string(record.Metadata),
		int64(record.OccurredAt), int64(record.OccurredAt))
	if err != nil {
		t.Fatalf("seed = %v, want nil", err)
	}
}

func relayOver(pool *pgxpool.Pool, publisher Publisher) Relay {
	clock := systemClock{}
	return Relay{
		Store:       postgres.NewOutboxStore(pool, clock),
		Publisher:   publisher,
		Clock:       clock,
		Source:      "urn:dmpf:orders",
		MaxAttempts: 3,
		Lease:       time.Minute,
		Backoff:     func(int) time.Duration { return time.Second },
	}
}

// OBX-07: publishing happens outside any database transaction. With a
// single-connection pool, a probe that can still acquire while Publish is
// blocked is the proof that the relay released the connection at commit.
func TestPublishingHoldsNoConnection(t *testing.T) {
	pool := openSingleConnPool(t)
	ctx := context.Background()
	seedOutboxRow(t, pool)

	publisher := newBlockingPublisher()
	relay := relayOver(pool, publisher)

	claimed, err := relay.Store.Claim(ctx, "claim-a", 10, relay.Lease)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("Claim() = %v, %d rows; want nil, 1", err, len(claimed))
	}

	type result struct {
		disposition disposition
		err         error
	}
	done := make(chan result, 1)
	go func() {
		got, err := relay.deliver(ctx, claimed[0])
		done <- result{got, err}
	}()

	<-publisher.entered
	if !probeCanAcquire(t, pool) {
		t.Fatal("a connection is held while Publish is in flight, which OBX-07 forbids")
	}
	close(publisher.release)

	got := <-done
	if got.err != nil {
		t.Fatalf("deliver() = %v, want nil", got.err)
	}
	if got.disposition != dispositionPublished {
		t.Fatalf("disposition = %v, want published", got.disposition)
	}
}

// The red control for the test above. Without it a probe that always succeeded
// would prove nothing: this holds a transaction open on purpose and requires
// the probe to fail.
func TestTheConnectionProbeFailsWhenAConnectionIsHeld(t *testing.T) {
	pool := openSingleConnPool(t)
	ctx := context.Background()
	seedOutboxRow(t, pool)

	publisher := newBlockingPublisher()
	relay := relayOver(pool, publisher)

	claimed, err := relay.Store.Claim(ctx, "claim-a", 10, relay.Lease)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("Claim() = %v, %d rows; want nil, 1", err, len(claimed))
	}

	held, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() = %v, want nil", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = relay.deliver(ctx, claimed[0])
	}()

	<-publisher.entered
	if probeCanAcquire(t, pool) {
		t.Fatal("the probe acquired a connection while one was deliberately held: it cannot detect a leak")
	}
	close(publisher.release)

	// deliver blocks on the transition until the held transaction lets go, so
	// the rollback has to come before the wait.
	_ = held.Rollback(ctx)
	<-done
}
