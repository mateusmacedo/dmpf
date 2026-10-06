package postgres

import (
	"context"
	"crypto/sha256"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const insertQuarantine = `
INSERT INTO quarantine (consumer_name, message_id, reason, envelope, envelope_digest, last_error, contained_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (consumer_name, envelope_digest) DO NOTHING`

var _ = declare(insertQuarantine, "INSERT", "quarantine")

const keyContainmentReason = "dmpf.containment.reason"

// NewQuarantine takes the pool, not a transaction: containment is outside any
// unit of work so the raw envelope is persisted even when the business
// transaction rolls back (GAR-07). The caller sanitizes errors (ERR-20, ERR-21).
func NewQuarantine(pool *pgxpool.Pool, opts ...QuarantineOption) ports.Containment {
	q := quarantine{pool: pool}
	for _, opt := range opts {
		opt(&q)
	}
	q.logger = loggerOf(q.logs)
	return q
}

type QuarantineOption func(*quarantine)

func WithQuarantineLoggerProvider(provider log.LoggerProvider) QuarantineOption {
	return func(q *quarantine) {
		q.logs = provider
	}
}

type quarantine struct {
	pool   *pgxpool.Pool
	logs   log.LoggerProvider
	logger *slog.Logger
}

func (q quarantine) Quarantine(ctx context.Context, c ports.Contained) error {
	if c.Consumer == "" || c.Reason == "" || len(c.Envelope) == 0 {
		return ErrInvalidContainment
	}

	var lastError *string
	if c.Error != "" {
		lastError = &c.Error
	}

	digest := sha256.Sum256(c.Envelope)
	if _, err := q.pool.Exec(ctx, insertQuarantine,
		c.Consumer, string(c.MessageID), string(c.Reason), c.Envelope, digest[:], lastError, int64(c.At)); err != nil {
		return err
	}
	attributes := []slog.Attr{slog.String(keyContainmentReason, string(c.Reason))}
	if c.MessageID != "" {
		attributes = append(attributes, slog.String(string(semconv.MessagingMessageIDKey), string(c.MessageID)))
	}
	q.logger.LogAttrs(ctx, slog.LevelWarn, "message contained", attributes...)
	return nil
}
