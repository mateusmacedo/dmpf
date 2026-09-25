//go:build integration

package appkit

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/provider"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

// Tables of this context, which the kit cannot know: it resets the kernel
// tables and the ones named here.
var Tables = []string{"orders"}

// PoolOptions is the database this context's suites run in.
var PoolOptions = pg.Options{
	Project:      "orders",
	Capabilities: []postgres.Capability{postgres.Outbox},
	Schemas:      []string{provider.Schema},
	Tables:       Tables,
}

func OpenPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	return pg.OpenPool(t, PoolOptions)
}

// Harness is the application service composed over the Postgres the suite
// runs against, with the clock and identifiers the test injects (KIT-07).
type Harness struct {
	Service application.Service
	Pool    *pgxpool.Pool
}

// NewOrders composes the service the way the composition root does, minus the
// instrumentation: a harness observes effects, not telemetry.
func NewOrders(t testing.TB, clock ports.Clock, ids ports.IDGenerator) Harness {
	t.Helper()
	pool := OpenPool(t)
	return Harness{
		Service: application.Service{
			UoW:       postgres.NewUnitOfWork(pool, bind),
			Reader:    provider.NewOrderReader(postgres.NewReadPool(pool)),
			Clock:     clock,
			IDs:       ids,
			Authorize: kernel.AllowAll[application.Operation](),
			ItemLimit: app.DefaultItemLimit,
		},
		Pool: pool,
	}
}

func bind(tx *postgres.Tx) application.Resources {
	return application.Resources{
		Orders: provider.NewOrderRepository(tx),
		Outbox: tx.Outbox(provider.Mapper{}),
	}
}

// Enqueued is one outbox record as the drain will read it, which is the effect
// edge of a producing context.
type Enqueued struct {
	MessageID        string
	MessageType      string
	AggregateVersion int64
	Destination      string
	Status           string
}

// Outbox reads what the use case left for the relay, ordered as it was
// enqueued, so a test asserts the sequence and not just the presence.
func (h Harness) Outbox(t testing.TB) []Enqueued {
	t.Helper()
	const query = `SELECT message_id, message_type, aggregate_version, destination, status
		FROM outbox ORDER BY id`

	rows, err := h.Pool.Query(context.Background(), query)
	if err != nil {
		t.Fatalf("appkit.Outbox: %v", err)
	}
	defer rows.Close()

	var out []Enqueued
	for rows.Next() {
		var e Enqueued
		if err := rows.Scan(&e.MessageID, &e.MessageType, &e.AggregateVersion, &e.Destination, &e.Status); err != nil {
			t.Fatalf("appkit.Outbox: scan: %v", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("appkit.Outbox: %v", err)
	}
	return out
}
