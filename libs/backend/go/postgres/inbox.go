package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// lockNotAvailable is the SQLSTATE PostgreSQL raises when lock_timeout expires.
const lockNotAvailable = "55P03"

const insertInbox = `
INSERT INTO inbox (consumer_name, message_id, message_type, payload_hash, received_at, processed_at, status)
VALUES ($1, $2, $3, $4, $5, $5, 'processed')
ON CONFLICT (consumer_name, message_id) DO NOTHING`

const selectInbox = `SELECT payload_hash, status FROM inbox WHERE consumer_name = $1 AND message_id = $2`

const updateInbox = `
UPDATE inbox SET status = $3, processed_at = $4, last_error = $5
WHERE consumer_name = $1 AND message_id = $2`

// Inbox binds a consumer and a wait ceiling to this open transaction. The
// provisional status written by Register is invisible outside the transaction
// and overwritten by Complete before commit (INB-02, INB-18). Under REPEATABLE
// READ or SERIALIZABLE the ON CONFLICT raises a serialization error and
// another realization is needed (FND-04 §6.2).
func (t *Tx) Inbox(consumer string, wait time.Duration) ports.Inbox {
	return &txInbox{tx: t, consumer: consumer, wait: wait}
}

type txInbox struct {
	tx       *Tx
	consumer string
	wait     time.Duration
}

func (i *txInbox) Register(ctx context.Context, r ports.Receipt) (ports.Reception, error) {
	if i.consumer == "" {
		return ports.Reception{}, ErrInboxConsumerRequired
	}
	if r.Consumer != i.consumer {
		return ports.Reception{}, ErrInboxConsumerMismatch
	}

	if i.wait > 0 {
		// WHY: SET does not accept parameters; the value is an integer in
		// milliseconds, so there is no injection surface. SET LOCAL scopes to
		// this transaction and never leaks to the pooled connection (RESEARCH §2).
		stmt := fmt.Sprintf("SET LOCAL lock_timeout = '%dms'", i.wait.Milliseconds())
		if _, err := i.tx.conn.Exec(ctx, stmt); err != nil {
			return ports.Reception{}, err
		}
	}

	tag, err := i.tx.conn.Exec(ctx, insertInbox,
		i.consumer, string(r.MessageID), r.MessageType, r.PayloadHash, int64(r.ReceivedAt))
	if err != nil {
		if isLockTimeout(err) {
			return ports.Reception{}, fmt.Errorf("%w: %w", ports.ErrRegisterTimeout, err)
		}
		return ports.Reception{}, err
	}

	if i.wait > 0 {
		// INB-17 is a ceiling on registering, not on the statements that follow
		// in the same transaction: a row lock in Save or Enqueue must not turn
		// into a 55P03 that Classify cannot recognise.
		if _, err := i.tx.conn.Exec(ctx, "SET LOCAL lock_timeout = DEFAULT"); err != nil {
			return ports.Reception{}, err
		}
	}

	if tag.RowsAffected() == 1 {
		return ports.FirstReception(&pending{tx: i.tx, consumer: i.consumer, messageID: r.MessageID}), nil
	}

	var (
		storedHash   string
		storedStatus string
	)
	switch err := i.tx.conn.QueryRow(ctx, selectInbox, i.consumer, string(r.MessageID)).Scan(&storedHash, &storedStatus); {
	case errors.Is(err, pgx.ErrNoRows):
		return ports.Reception{}, fmt.Errorf("postgres: inbox row absent after conflict on (%s, %s)", i.consumer, r.MessageID)
	case err != nil:
		return ports.Reception{}, err
	}

	if storedHash != r.PayloadHash {
		return ports.CollisionReception(), nil
	}
	if storedStatus == ports.StatusProcessed.String() {
		return ports.ProcessedReception(), nil
	}
	return ports.RejectedReception(), nil
}

type pending struct {
	tx        *Tx
	consumer  string
	messageID ports.MessageID
	completed bool
}

func (p *pending) Complete(ctx context.Context, c ports.Completion) error {
	if p.completed {
		return ErrAlreadyCompleted
	}
	if c.Status != ports.StatusProcessed && c.Status != ports.StatusRejected {
		return ErrInvalidCompletion
	}

	var lastError *string
	if c.LastError != "" {
		lastError = &c.LastError
	}

	tag, err := p.tx.conn.Exec(ctx, updateInbox,
		p.consumer, string(p.messageID), c.Status.String(), int64(c.At), lastError)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("postgres: inbox update affected %d rows, want 1", tag.RowsAffected())
	}

	p.completed = true
	return nil
}

func (p *pending) Completed() bool { return p.completed }

func isLockTimeout(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == lockNotAvailable
}
