package orderspg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// NewReader serves the read side without the write side (UOW-11): it takes the
// pool rather than a Tx because a query must not open a transaction, so the
// SELECT runs on a pooled connection in autocommit.
func NewReader(pool *pgxpool.Pool) dmpfports.Reader[orders.OrderID, orders.Snapshot] {
	return reader{pool: pool}
}

type reader struct{ pool *pgxpool.Pool }

func (r reader) Load(ctx context.Context, id orders.OrderID) (orders.Snapshot, dmpfports.Version, error) {
	return load(ctx, r.pool, id)
}
