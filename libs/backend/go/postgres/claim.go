package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Claimed is one outbox record under a live lease, carrying the columns the
// relay needs to assemble the envelope (ENV-14) plus the claim identity that
// every later transition is conditioned on (OBX-10).
type Claimed struct {
	ID               int64
	MessageID        string
	MessageType      string
	SchemaVersion    string
	AggregateType    string
	AggregateID      string
	AggregateVersion int64
	PartitionKey     string
	Destination      string
	Payload          []byte
	PayloadHash      string
	Metadata         []byte
	OccurredAt       ports.Instant
	AttemptCount     int
	LockedBy         string
}

// The eligibility predicate is OBX-09 plus D5. locked_until IS NULL is what
// keeps a record released by OBX-18 from being stranded in publishing forever:
// SQL evaluates locked_until <= $1 as unknown for a released lease, so without
// this clause the recomputed backoff would be inert.
//
// The three metadata keys are ENV-08 attributes with no column of their own.
// The test is the same one the envelope assembly applies — a non-empty JSON
// string, not merely a present key: a record carrying "" or a number would
// pass a presence check, fail assembly, and be driven to failed, which is the
// exact outcome D5 exists to avoid.
const claimStatement = `
WITH eligible AS (
	SELECT id
	  FROM dmpf_outbox
	 WHERE available_at <= $1
	   AND ( status = 'pending'
	      OR ( status = 'publishing'
	           AND (locked_until IS NULL OR locked_until <= $1) ) )
	   AND jsonb_typeof(metadata -> 'correlationid') = 'string' AND metadata ->> 'correlationid' <> ''
	   AND jsonb_typeof(metadata -> 'causationid') = 'string' AND metadata ->> 'causationid' <> ''
	   AND jsonb_typeof(metadata -> 'traceparent') = 'string' AND metadata ->> 'traceparent' <> ''
	 ORDER BY available_at, id
	 LIMIT $2
	 FOR UPDATE SKIP LOCKED
)
UPDATE dmpf_outbox AS o
   SET status = 'publishing',
       locked_by = $3,
       locked_until = $4,
       attempt_count = o.attempt_count + 1
  FROM eligible
 WHERE o.id = eligible.id
RETURNING o.id, o.message_id, o.message_type, o.schema_version,
          o.aggregate_type, o.aggregate_id, o.aggregate_version,
          o.partition_key, o.destination, o.payload, o.payload_hash, o.metadata,
          o.occurred_at, o.attempt_count`

const (
	// locked_until is cleared with the rest: a published record holds no claim,
	// and leaving the deadline behind would show a live lease on a row nobody
	// is draining. locked_by stays as the audit trail of which claim published.
	markPublishedStatement = `
UPDATE dmpf_outbox SET status = 'published', published_at = $3, locked_until = NULL, last_error = NULL
 WHERE id = $1 AND locked_by = $2`

	// status stays publishing: OBX-04 forbids writing pending over it, and the
	// return to the pool is by deadline comparison, not by state.
	//
	// GREATEST guards the schema's available_at >= occurred_at check: an event
	// whose occurred_at is ahead of this relay's clock would otherwise make the
	// write fail with 23514, stranding the record in publishing until the lease
	// runs out. Clamping to occurred_at means the record is drainable at once,
	// which is what a backoff already in the past asks for anyway.
	//
	// An empty lastError keeps whatever was there: the graceful release ends a
	// claim without a failure of its own, and overwriting the reason the last
	// delivery failed would erase the only diagnosis the row carries.
	rescheduleStatement = `
UPDATE dmpf_outbox
   SET available_at = GREATEST($3, occurred_at),
       locked_until = NULL,
       last_error = COALESCE(NULLIF($4, ''), last_error)
 WHERE id = $1 AND locked_by = $2`

	failStatement = `
UPDATE dmpf_outbox SET status = 'failed', locked_until = NULL, last_error = $3
 WHERE id = $1 AND locked_by = $2`
)

// OutboxStore is the drain side of the outbox: the claim and the three
// conditional transitions of FND-04 §5.4. It is a struct so the relay can
// declare its own interface over it (RFC §7.5 keeps the loop in app and the
// technology here).
type OutboxStore struct {
	pool  *pgxpool.Pool
	clock ports.Clock
}

func NewOutboxStore(pool *pgxpool.Pool, clock ports.Clock) OutboxStore {
	return OutboxStore{pool: pool, clock: clock}
}

// Claim acquires up to limit eligible records under a fresh identity, writing
// the four fields of OBX-16 in one commit. The lease instant is read inside the
// transaction, after the connection is held: reading it before would date the
// lease by however long the pool made the caller wait.
func (s OutboxStore) Claim(ctx context.Context, claimID string, limit int, lease time.Duration) ([]Claimed, error) {
	if s.pool == nil || s.clock == nil {
		return nil, ErrIncompleteStore
	}
	if claimID == "" {
		return nil, ErrClaimIdentityRequired
	}
	if limit <= 0 {
		return nil, ErrInvalidBatchSize
	}
	if lease <= 0 {
		return nil, ErrInvalidLease
	}

	var claimed []Claimed
	uow := NewUnitOfWork(s.pool, func(tx *Tx) *Tx { return tx })
	err := uow.Within(ctx, func(ctx context.Context, tx *Tx) error {
		now := s.clock.Now()
		rows, err := tx.conn.Query(ctx, claimStatement, int64(now), limit, claimID, int64(now)+int64(lease))
		if err != nil {
			return err
		}
		defer rows.Close()

		claimed, err = collectClaimed(rows, claimID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func collectClaimed(rows pgx.Rows, claimID string) ([]Claimed, error) {
	var claimed []Claimed
	for rows.Next() {
		c := Claimed{LockedBy: claimID}
		var occurredAt int64
		if err := rows.Scan(
			&c.ID, &c.MessageID, &c.MessageType, &c.SchemaVersion,
			&c.AggregateType, &c.AggregateID, &c.AggregateVersion,
			&c.PartitionKey, &c.Destination, &c.Payload, &c.PayloadHash, &c.Metadata,
			&occurredAt, &c.AttemptCount,
		); err != nil {
			return nil, err
		}
		c.OccurredAt = ports.Instant(occurredAt)
		claimed = append(claimed, c)
	}
	return claimed, rows.Err()
}

// MarkPublished is outcome 3a. It reports how many rows it changed: zero means
// the claim was replaced while the message was in flight (OBX-10), and the
// caller records the fact instead of republishing.
func (s OutboxStore) MarkPublished(ctx context.Context, id int64, claimID string) (int64, error) {
	if s.clock == nil {
		return 0, ErrIncompleteStore
	}
	return s.transition(ctx, markPublishedStatement, id, claimID, int64(s.clock.Now()))
}

// Reschedule is outcome 3b: the backoff lands on available_at and the lease is
// released in the same commit, so the next claim obeys the backoff rather than
// whatever was left of the lease (OBX-18).
func (s OutboxStore) Reschedule(ctx context.Context, id int64, claimID string, availableAt ports.Instant, lastError string) (int64, error) {
	return s.transition(ctx, rescheduleStatement, id, claimID, int64(availableAt), lastError)
}

// Fail is outcome 3c: terminal for the automatic cycle (OBX-06). The lease is
// released too, so a failed record carries no claim anybody could mistake for live.
func (s OutboxStore) Fail(ctx context.Context, id int64, claimID string, lastError string) (int64, error) {
	return s.transition(ctx, failStatement, id, claimID, lastError)
}

func (s OutboxStore) transition(ctx context.Context, statement string, id int64, claimID string, args ...any) (int64, error) {
	if s.pool == nil {
		return 0, ErrIncompleteStore
	}
	if claimID == "" {
		return 0, ErrClaimIdentityRequired
	}

	tag, err := s.pool.Exec(ctx, statement, append([]any{id, claimID}, args...)...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
