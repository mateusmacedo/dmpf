// comment-discipline-ok-file: arquivo de declarações; o godoc de txInbox documenta por que INB-06 é trivial nesta realização, dentro do limite de 3 linhas.

package memory

import (
	"context"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

type inboxKey struct {
	consumer string
	id       dmpfports.MessageID
}

type inboxRow struct {
	hash      string
	status    dmpfports.Status
	lastError string
}

// txInbox is the transactional Inbox bound to one consumer. Store.txMu
// serializes the whole callback (tx.go:60), so Register never waits on a
// key here: INB-06 holds without any lock of its own.
type txInbox struct {
	tx       *Tx
	consumer string
}

func (i txInbox) Register(_ context.Context, r dmpfports.Receipt) (dmpfports.Reception, error) {
	if i.consumer == "" || r.Consumer == "" {
		return dmpfports.Reception{}, ErrInboxConsumerRequired
	}
	if r.Consumer != i.consumer {
		return dmpfports.Reception{}, ErrInboxConsumerMismatch
	}

	key := inboxKey{consumer: i.consumer, id: r.MessageID}
	if existing, ok := i.tx.inbox[key]; ok {
		if existing.hash != r.PayloadHash {
			return dmpfports.CollisionReception(), nil
		}
		if existing.status == dmpfports.StatusProcessed {
			return dmpfports.ProcessedReception(), nil
		}
		return dmpfports.RejectedReception(), nil
	}
	return dmpfports.FirstReception(&memoryPending{tx: i.tx, key: key, hash: r.PayloadHash}), nil
}

var _ dmpfports.Inbox = txInbox{}

// memoryPending is the write half of a first reception: Complete stages the
// row in the open transaction, visible to the store only at commit.
type memoryPending struct {
	tx        *Tx
	key       inboxKey
	hash      string
	completed bool
}

func (p *memoryPending) Complete(_ context.Context, c dmpfports.Completion) error {
	if p.completed {
		return ErrAlreadyCompleted
	}
	if c.Status != dmpfports.StatusProcessed && c.Status != dmpfports.StatusRejected {
		return ErrInvalidCompletion
	}
	p.tx.inbox[p.key] = inboxRow{hash: p.hash, status: c.Status, lastError: c.LastError}
	p.completed = true
	return nil
}

func (p *memoryPending) Completed() bool { return p.completed }

var _ dmpfports.Pending = (*memoryPending)(nil)
