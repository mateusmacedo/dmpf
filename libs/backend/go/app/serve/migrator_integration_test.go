//go:build integration

package serve

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	lognoop "go.opentelemetry.io/otel/log/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

func TestAwaitGrantsRefusesTheStartWhenTheGrantsDoNotArriveInTime(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), pg.RuntimeDSN(t, "serve"))
	if err != nil {
		t.Fatalf("pgxpool.New() = %v", err)
	}
	t.Cleanup(pool.Close)

	err = awaitGrants(context.Background(), pool, lognoop.NewLoggerProvider(), 300*time.Millisecond, postgres.KernelGrants(postgres.Outbox))

	if !errors.Is(err, ErrRuntimeUngranted) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("awaitGrants() without the grants = %v, want ErrRuntimeUngranted at the deadline", err)
	}
}

func TestAwaitGrantsLeavesTheStopOfTheCallerAsIs(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), pg.RuntimeDSN(t, "serve"))
	if err != nil {
		t.Fatalf("pgxpool.New() = %v", err)
	}
	t.Cleanup(pool.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err = awaitGrants(ctx, pool, lognoop.NewLoggerProvider(), time.Minute, postgres.KernelGrants(postgres.Outbox))

	if errors.Is(err, ErrRuntimeUngranted) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("awaitGrants() past the deadline of the caller = %v, want that deadline, not ErrRuntimeUngranted", err)
	}
}
