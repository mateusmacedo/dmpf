package dmpfpostgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

const selectSignals = `SELECT reason, count(*) FROM dmpf_quarantine WHERE consumer_name = $1 GROUP BY reason`

// Signals is the aggregated health snapshot of one consumer's quarantine,
// derived from the reason column (GAR-12).
type Signals struct {
	QuarantineDepth   int64
	TerminalFailures  int64
	Collisions        int64
	AttemptsExhausted int64
	InvalidEnvelopes  int64
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
		}
	}
	return s, rows.Err()
}
