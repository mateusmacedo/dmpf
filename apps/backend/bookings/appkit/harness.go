//go:build integration

package appkit

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

// Tables of this context, which the kit cannot know: it resets the shared
// DMPF tables and the ones named here.
var Tables = []string{"bookings_booking", "bookings_resource"}

// Harness is the application service composed over the Postgres the suite
// runs against, with the clock and identifiers the test injects (KIT-07).
type Harness struct {
	Service application.Service
	Pool    *pgxpool.Pool
}

// NewBookings composes the service the way the composition root does, minus
// the instrumentation: a harness observes effects, not telemetry.
func NewBookings(t testing.TB, clock ports.Clock, ids ports.IDGenerator) Harness {
	t.Helper()
	pool := pg.OpenPool(t, Tables...)
	if err := provider.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("appkit.NewBookings: Migrate: %v", err)
	}
	pg.ResetTables(t, pool, Tables...)
	return Harness{
		Service: application.Service{
			UoW:            postgres.NewUnitOfWork(pool, bind),
			Reader:         provider.NewBookingReader(pool),
			ResourceReader: provider.NewBookingsByResourceReader(pool),
			Clock:          clock,
			IDs:            ids,
			Authorize:      kernel.AllowAllWithContext[application.Operation](),
		},
		Pool: pool,
	}
}

func bind(tx *postgres.Tx) application.Resources {
	return application.Resources{
		Bookings:  provider.NewBookingRepository(tx),
		Resources: provider.NewResourceRepository(tx),
		Outbox:    tx.Outbox(provider.Mapper{}),
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
		FROM dmpf_outbox ORDER BY id`

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
