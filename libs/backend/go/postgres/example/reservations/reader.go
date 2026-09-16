package reservationspg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// NewReader serves the read side without the write side (UOW-11): it takes the
// pool rather than a Tx because a query must not open a transaction, so the
// SELECT runs on a pooled connection in autocommit.
func NewReader(pool *pgxpool.Pool) ports.Reader[reservations.OrderID, reservations.Snapshot] {
	return reader{pool: pool}
}

type reader struct{ pool *pgxpool.Pool }

func (r reader) Load(ctx context.Context, id reservations.OrderID) (reservations.Snapshot, ports.Version, error) {
	return load(ctx, r.pool, id)
}
