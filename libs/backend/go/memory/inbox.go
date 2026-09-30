// comment-discipline-ok-file: arquivo de declarações; o godoc de txInbox documenta por que INB-06 é trivial nesta realização, dentro do limite de 3 linhas.

package memory

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type inboxKey struct {
	consumer string
	id       ports.MessageID
}

type inboxRow struct {
	hash      string
	status    ports.Status
	lastError string
	outcome   []byte
	expiresAt ports.Instant
}

// CommandInbox is the Inbox of a context's commands: the key is scoped to the
// carrier's tenant, and an entry at or past its expiry counts as absent.
func (t *Tx) CommandInbox(consumer string) ports.Inbox {
	return txInbox{tx: t, consumer: consumer, commands: true}
}

// txInbox is the transactional Inbox bound to one consumer. Store.txMu
// serializes the whole callback (tx.go:59), so Register never waits on a
// key here: INB-06 holds without any lock of its own.
type txInbox struct {
	tx       *Tx
	consumer string
	commands bool
}

func (i txInbox) Register(ctx context.Context, r ports.Receipt) (ports.Reception, error) {
	if i.consumer == "" || r.Consumer == "" {
		return ports.Reception{}, ErrInboxConsumerRequired
	}
	if r.Consumer != i.consumer {
		return ports.Reception{}, ErrInboxConsumerMismatch
	}

	id := r.MessageID
	if i.commands {
		if r.ExpiresAt <= r.ReceivedAt {
			return ports.Reception{}, ErrCommandExpiryRequired
		}
		scoped, err := scopedKey(ctx, r.MessageID)
		if err != nil {
			return ports.Reception{}, err
		}
		id = ports.MessageID(string(scoped.tenant) + "/" + string(r.MessageID))
	}

	key := inboxKey{consumer: i.consumer, id: id}
	if existing, ok := i.tx.inbox[key]; ok && !expired(existing, r.ReceivedAt) {
		if existing.hash != r.PayloadHash {
			return ports.CollisionReception(), nil
		}
		if existing.status == ports.StatusProcessed {
			return ports.ProcessedReception().WithStored(existing.outcome), nil
		}
		return ports.RejectedReception().WithStored(existing.outcome), nil
	}
	return ports.FirstReception(&memoryPending{tx: i.tx, key: key, hash: r.PayloadHash, expiresAt: r.ExpiresAt}), nil
}

func expired(row inboxRow, at ports.Instant) bool {
	return row.expiresAt != 0 && row.expiresAt <= at
}

var _ ports.Inbox = txInbox{}

// memoryPending is the write half of a first reception: Complete stages the
// row in the open transaction, visible to the store only at commit.
type memoryPending struct {
	tx        *Tx
	key       inboxKey
	hash      string
	expiresAt ports.Instant
	completed bool
}

func (p *memoryPending) Complete(_ context.Context, c ports.Completion) error {
	if p.completed {
		return ErrAlreadyCompleted
	}
	if c.Status != ports.StatusProcessed && c.Status != ports.StatusRejected {
		return ErrInvalidCompletion
	}
	p.tx.inbox[p.key] = inboxRow{
		hash:      p.hash,
		status:    c.Status,
		lastError: c.LastError,
		outcome:   append([]byte(nil), c.Outcome...),
		expiresAt: p.expiresAt,
	}
	p.completed = true
	return nil
}

func (p *memoryPending) Completed() bool { return p.completed }

var _ ports.Pending = (*memoryPending)(nil)
