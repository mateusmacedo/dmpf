package postgres

import (
	"context"
	"crypto/sha256"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const insertQuarantine = `
INSERT INTO quarantine (consumer_name, message_id, reason, envelope, envelope_digest, last_error, contained_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (consumer_name, envelope_digest) DO NOTHING`

// NewQuarantine takes the pool, not a transaction: containment is outside any
// unit of work so the raw envelope is persisted even when the business
// transaction rolls back (GAR-07). The caller sanitizes errors (ERR-20, ERR-21).
func NewQuarantine(pool *pgxpool.Pool) ports.Containment {
	return quarantine{pool: pool}
}

type quarantine struct{ pool *pgxpool.Pool }

func (q quarantine) Quarantine(ctx context.Context, c ports.Contained) error {
	if c.Consumer == "" || c.Reason == "" || len(c.Envelope) == 0 {
		return ErrInvalidContainment
	}

	var lastError *string
	if c.Error != "" {
		lastError = &c.Error
	}

	digest := sha256.Sum256(c.Envelope)
	_, err := q.pool.Exec(ctx, insertQuarantine,
		c.Consumer, string(c.MessageID), string(c.Reason), c.Envelope, digest[:], lastError, int64(c.At))
	return err
}
