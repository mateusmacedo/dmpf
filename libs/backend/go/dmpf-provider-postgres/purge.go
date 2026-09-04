package dmpfpostgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

const deletePublished = `DELETE FROM dmpf_outbox WHERE status = 'published' AND published_at < $1`

// Purge is the evidence a purge leaves behind: how many rows went and up to
// which instant. Returning it rather than a bare count is what lets an operator
// tell "nothing to purge" from "purged with the wrong cutoff" (OBX-17).
type Purge struct {
	Count  int64
	Before dmpfports.Instant
}

// PurgePublished takes the pool, not a transaction: retention is housekeeping
// outside any unit of work, and it must never share a transaction with business
// state. Only published rows go — pending, publishing and failed are still owed
// to someone.
func PurgePublished(ctx context.Context, pool *pgxpool.Pool, before dmpfports.Instant) (Purge, error) {
	tag, err := pool.Exec(ctx, deletePublished, int64(before))
	if err != nil {
		return Purge{}, err
	}
	return Purge{Count: tag.RowsAffected(), Before: before}, nil
}
