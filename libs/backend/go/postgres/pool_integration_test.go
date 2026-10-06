//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const insertPending = `
INSERT INTO outbox (
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

var semconvDurationBoundaries = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func TestTheWaitForASaturatedPoolIsMeasured(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(ctx) })

	dsn := pg.DSN(t, "postgres")
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig() = %v", err)
	}
	conn := config.ConnConfig
	name := poolName(conn.Host + ":" + strconv.Itoa(int(conn.Port)) + "/" + conn.Database)

	pool, err := postgres.NewPool(ctx, dsn+"&pool_max_conns=1", noop.NewTracerProvider().Tracer("test"),
		postgres.WithMeterProvider(provider))
	if err != nil {
		t.Fatalf("NewPool() = %v, want nil", err)
	}
	t.Cleanup(pool.Close)

	held, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire(first) = %v, want nil", err)
	}
	t.Cleanup(held.Release)

	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if _, err := pool.Acquire(timeoutCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire(saturated, 20ms) = %v, want DeadlineExceeded", err)
	}

	acquired := make(chan error, 1)
	go func() {
		conn, err := pool.Acquire(ctx)
		if err == nil {
			conn.Release()
		}
		acquired <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	for sumPoint(t, collect(t, reader)["db.client.connection.pending_requests"], name) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("pending_requests never reached 1 while the second acquire waited")
		}
		time.Sleep(5 * time.Millisecond)
	}
	got := collect(t, reader)
	if v := sumPoint(t, got["db.client.connection.count"], name, connectionState("used")); v != 1 {
		t.Errorf("count{used} = %d, want 1 while the first connection is held", v)
	}
	if v := sumPoint(t, got["db.client.connection.max"], name); v != 1 {
		t.Errorf("max = %d, want 1", v)
	}
	if v := sumPoint(t, got["db.client.connection.timeouts"], name); v != 1 {
		t.Errorf("timeouts = %d, want the one acquire that hit its deadline", v)
	}

	const heldFor = 50 * time.Millisecond
	time.Sleep(heldFor)
	held.Release()
	if err := <-acquired; err != nil {
		t.Fatalf("Acquire(second) = %v, want nil", err)
	}

	got = collect(t, reader)
	histogram, ok := got["db.client.connection.wait_time"].(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("wait_time = %T, want a float64 histogram", got["db.client.connection.wait_time"])
	}
	want := attribute.NewSet(name)
	waited := false
	for _, point := range histogram.DataPoints {
		if !point.Attributes.Equals(&want) {
			t.Errorf("wait_time point with %v, want only %v", point.Attributes.ToSlice(), want.ToSlice())
			continue
		}
		if point.Count != 2 {
			t.Errorf("wait_time count = %d, want the 2 acquires that succeeded", point.Count)
		}
		if !slices.Equal(point.Bounds, semconvDurationBoundaries) {
			t.Errorf("wait_time bounds = %v, want the semconv advisory %v (RF-D4)", point.Bounds, semconvDurationBoundaries)
		}
		if longest, defined := point.Max.Value(); defined && longest >= heldFor.Seconds() {
			waited = true
		}
	}
	if !waited {
		t.Fatalf("wait_time = %+v, want a sample of at least %v for the acquire that waited", histogram.DataPoints, heldFor)
	}
	if v := sumPoint(t, got["db.client.connection.pending_requests"], name); v != 0 {
		t.Errorf("pending_requests = %d, want 0 once every acquire returned", v)
	}
}
