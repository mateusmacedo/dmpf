package dmpfpostgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

const selectSignals = `SELECT reason, count(*) FROM dmpf_quarantine WHERE consumer_name = $1 GROUP BY reason`

// failed is excluded from pending on purpose: OBX-06 makes it terminal, so
// counting it as backlog would report work no cycle is going to do. It gets a
// counter of its own instead.
//
// The four aggregates come from one sequential scan of the whole table, which
// includes every published row: purging those is OBX-17 and is not implemented
// yet, so the cost of this reading grows with the total history rather than
// with the backlog. Call it on a schedule, never per drained record.
const selectOutboxSignals = `
SELECT
	count(*) FILTER (WHERE status IN ('pending', 'publishing')),
	coalesce(min(occurred_at) FILTER (WHERE status IN ('pending', 'publishing')), 0),
	coalesce(sum(attempt_count) FILTER (WHERE status IN ('pending', 'publishing')), 0),
	count(*) FILTER (WHERE status = 'failed')
  FROM dmpf_outbox`

// Signals is the aggregated health snapshot of one consumer's quarantine,
// derived from the reason column (GAR-12).
type Signals struct {
	QuarantineDepth   int64
	TerminalFailures  int64
	Collisions        int64
	AttemptsExhausted int64
	InvalidEnvelopes  int64
	// Other keeps the depth auditable: a reason the port does not enumerate is
	// counted here instead of vanishing from the breakdown.
	Other int64
}

// InboxSignals returns the current quarantine signals for one consumer.
func InboxSignals(ctx context.Context, pool *pgxpool.Pool, consumer string) (Signals, error) {
	if consumer == "" {
		return Signals{}, ErrInboxConsumerRequired
	}

	rows, err := pool.Query(ctx, selectSignals, consumer)
	if err != nil {
		return Signals{}, err
	}
	defer rows.Close()

	var s Signals
	for rows.Next() {
		var (
			reason string
			count  int64
		)
		if err := rows.Scan(&reason, &count); err != nil {
			return Signals{}, err
		}
		s.QuarantineDepth += count
		switch dmpfports.Reason(reason) {
		case dmpfports.ReasonTerminalFailure:
			s.TerminalFailures = count
		case dmpfports.ReasonCollision:
			s.Collisions = count
		case dmpfports.ReasonAttemptsExhausted:
			s.AttemptsExhausted = count
		case dmpfports.ReasonInvalidEnvelope:
			s.InvalidEnvelopes = count
		default:
			s.Other += count
		}
	}
	return s, rows.Err()
}

// OutboxHealth is the drain-side snapshot OBX-12 requires the relay to expose.
// Naming the metrics, their units and their thresholds is FND-08's under
// ANC-06, and the OpenTelemetry binding is KRN-09's; this is the raw reading.
type OutboxHealth struct {
	// Pending counts the records still owed to a broker: pending and
	// publishing, never failed.
	Pending int64

	// Lag is how long the oldest of those has been waiting, in nanoseconds,
	// and is zero when there is nothing pending.
	Lag int64

	// Attempts is how many acquisitions those pending records have already
	// cost, and Failures how many gave up (OBX-06).
	Attempts int64
	Failures int64
}

// OutboxSignals reads the four signals at the instant the clock reports. It
// takes the clock rather than the wall time so lag is measured on the same
// clock the relay claims with.
func OutboxSignals(ctx context.Context, pool *pgxpool.Pool, clock dmpfports.Clock) (OutboxHealth, error) {
	if pool == nil || clock == nil {
		return OutboxHealth{}, ErrIncompleteStore
	}

	var (
		health OutboxHealth
		oldest int64
		now    = int64(clock.Now())
	)
	err := pool.QueryRow(ctx, selectOutboxSignals).
		Scan(&health.Pending, &oldest, &health.Attempts, &health.Failures)
	if err != nil {
		return OutboxHealth{}, err
	}

	if health.Pending > 0 && now > oldest {
		health.Lag = now - oldest
	}
	return health, nil
}

// OutboxSignals on the store is the same reading bound to the pool and clock
// the relay already holds.
func (s OutboxStore) OutboxSignals(ctx context.Context) (OutboxHealth, error) {
	return OutboxSignals(ctx, s.pool, s.clock)
}
