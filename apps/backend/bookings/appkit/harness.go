//go:build integration

package appkit

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

// Tables of this context, which the kit cannot know: it resets the shared
// DMPF tables and the ones named here.
var Tables = []string{"bookings", "resources"}

// PoolOptions is the database this context's suites run in.
var PoolOptions = pg.Options{
	Project:      "bookings",
	Capabilities: []postgres.Capability{postgres.Outbox, postgres.Inbox},
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

// NewBookings composes the service the way the composition root does, minus
// the instrumentation: a harness observes effects, not telemetry.
func NewBookings(t testing.TB, clock ports.Clock, ids ports.IDGenerator) Harness {
	t.Helper()
	pool := OpenPool(t)
	policy, err := kernelapp.IdempotencyPolicy(time.Second, 24*time.Hour)
	if err != nil {
		t.Fatalf("appkit.NewBookings: %v", err)
	}
	service := app.NewService(pool, clock, ids, app.Waits{Command: time.Second})
	service.Authorize = usecase.AllowAll[application.Operation]()
	service.Idempotency = policy
	return Harness{Service: service, Pool: pool}
}

// Outbox reads what the use case left for the relay, ordered as it was
// enqueued, so a test asserts the sequence and not just the presence.
func (h Harness) Outbox(t testing.TB) []pg.Enqueued {
	t.Helper()
	return pg.Outbox(t, h.Pool)
}
