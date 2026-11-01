package providerkit

import (
	"context"
	"strconv"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
)

type OutboxResources struct{ Outbox ports.Outbox }

type inboxResources struct{ Inbox ports.Inbox }

func MemoryUnitOfWork() UnitOfWorkSubject[OutboxResources] {
	store := memory.New()
	n := 0
	return UnitOfWorkSubject[OutboxResources]{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) OutboxResources { return OutboxResources{Outbox: tx.Outbox()} }),
		Write: func(ctx context.Context, res OutboxResources) error {
			n++
			return res.Outbox.Enqueue(ctx, memoryEntry(ports.MessageID("m-"+strconv.Itoa(n))))
		},
		Kept:             func() int { return len(store.Entries()) },
		ArmCommitFailure: store.FailNextCommit,
		Commits:          store.Commits,
	}
}

// MemoryInbox is the inbox subject over the in-memory realization. Store.txMu
// serializes every transaction, so the two-insert race has nothing to observe
// and is left to the Postgres realization.
func MemoryInbox() InboxSubject {
	store := memory.New()
	return InboxSubject{
		Within: func(ctx context.Context, consumer string, fn func(ctx context.Context, inbox ports.Inbox) error) error {
			uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) inboxResources { return inboxResources{Inbox: tx.Inbox(consumer)} })
			return uow.Within(ctx, func(ctx context.Context, res inboxResources) error { return fn(ctx, res.Inbox) })
		},
		ReadStatus:       store.InboxStatus,
		Concurrent:       false,
		ConsumerMismatch: memory.ErrInboxConsumerMismatch,
		Rows:             store.InboxRows,
	}
}

// Off the whole second, so a kit that compared retention dates finer than the
// second S3 keeps would fail on the fake too.
const kitObjectEpoch = ports.Instant(1_760_000_000_123_456_789)

// MemoryObjectStore is the object store subject over the in-memory fake: every
// Open shares one bucket, whose fake clock Advance moves.
func MemoryObjectStore() ObjectStoreSubject {
	fake := clock.New(kitObjectEpoch)
	bucket := memory.NewObjectBucket(fake)
	return ObjectStoreSubject{
		Open: func(decl ports.ObjectStoreDeclaration) (ports.ObjectStore, error) {
			store, err := memory.NewObjectStore(bucket, decl)
			if err != nil {
				return nil, err
			}
			return store, nil
		},
		Advance: func(nanos int64) error {
			fake.Advance(time.Duration(nanos))
			return nil
		},
		TenantUnresolved: memory.ErrTenantUnresolved,
		OtherEncryption:  ports.Encryption{Algorithm: "AES256"},
		PhysicalKeys:     func() ([]string, error) { return bucket.PhysicalKeys(), nil },
	}
}

func memoryEntry(id ports.MessageID) ports.OutboxEntry {
	return ports.OutboxEntry{
		MessageID: id, OccurredAt: 1,
		Intent:        ports.PublishIntent{Destination: "orders.events", PartitionKey: "o-1"},
		AggregateType: "orders.Order", AggregateID: "o-1", AggregateVersion: 1,
	}
}
