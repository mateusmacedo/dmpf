package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const deletePublished = `DELETE FROM dmpf_outbox WHERE status = 'published' AND published_at < $1`

// Purge is the evidence a purge leaves behind: how many rows went and up to
// which instant. Returning it rather than a bare count is what lets an operator
// tell "nothing to purge" from "purged with the wrong cutoff" (OBX-17).
type Purge struct {
	Count  int64
	Before ports.Instant
}

// PurgePublished takes the pool, not a transaction: retention is housekeeping
// outside any unit of work, and it must never share a transaction with business
// state. Only published rows go — pending, publishing and failed are still owed
// to someone.
func PurgePublished(ctx context.Context, pool *pgxpool.Pool, before ports.Instant) (Purge, error) {
	tag, err := pool.Exec(ctx, deletePublished, int64(before))
	if err != nil {
		return Purge{}, err
	}
	return Purge{Count: tag.RowsAffected(), Before: before}, nil
}

const deleteInbox = `DELETE FROM dmpf_inbox WHERE consumer_name = $1 AND processed_at < $2`

// InboxPurge is the evidence an inbox purge leaves behind, scoped to one
// consumer (INB-16).
type InboxPurge struct {
	Consumer string
	Removed  int64
	Before   ports.Instant
}

// PurgeInbox removes terminal inbox rows — processed and rejected alike — older
// than before, for one consumer.
// The retention invariant is the operator's responsibility (INB-14); this
// function provides the evidence for audit (INB-16).
func PurgeInbox(ctx context.Context, pool *pgxpool.Pool, consumer string, before ports.Instant) (InboxPurge, error) {
	if consumer == "" {
		return InboxPurge{}, ErrInboxConsumerRequired
	}
	tag, err := pool.Exec(ctx, deleteInbox, consumer, int64(before))
	if err != nil {
		return InboxPurge{}, err
	}
	return InboxPurge{Consumer: consumer, Removed: tag.RowsAffected(), Before: before}, nil
}
