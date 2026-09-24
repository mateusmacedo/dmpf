package relay

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// NewOverPostgres assembles the drain of FND-04 §5.4 over an outbox in
// Postgres. It runs in its own process, apart from the request path (BLK-02).
//
// The component names the process in the panic of the identity minting, so a
// crash says which relay lost its entropy source.
func NewOverPostgres(pool *pgxpool.Pool, publisher Publisher, component string, config Config) (Relay, error) {
	clock := idclock.SystemClock{}
	return New(postgres.NewOutboxStore(pool, clock), publisher, idclock.NewClaimIDs(component), clock, config)
}
