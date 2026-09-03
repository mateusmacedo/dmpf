package memory

import (
	"context"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// Tx is the working copy of one transaction: writes land here and reach the
// store only at commit.
type Tx struct {
	orders map[orders.OrderID]record
	outbox []dmpfports.OutboxEntry
}

// Orders is the transactional repository of the open transaction.
func (t *Tx) Orders() dmpfports.Repository[orders.OrderID, orders.Snapshot] {
	return txOrders{tx: t}
}

// Outbox is the transactional outbox of the open transaction.
func (t *Tx) Outbox() dmpfports.Outbox {
	return txOutbox{tx: t}
}

// NewUnitOfWork binds an open transaction to the resource set R that a use case
// declares. bind is written by the composition root, never here: provider →
// application is a forbidden cell, so this package cannot know R's shape.
func NewUnitOfWork[R any](store *Store, bind func(tx *Tx) R) dmpfports.UnitOfWork[R] {
	return unitOfWork[R]{store: store, bind: bind}
}

type unitOfWork[R any] struct {
	store *Store
	bind  func(tx *Tx) R
}

// Within holds the store's mutex for the whole callback, which serializes
// transactions and is why this realization proves atomicity but not isolation.
// A panic in fn unwinds without touching the store, so the discarded Tx is the
// rollback (ERR-22).
func (u unitOfWork[R]) Within(ctx context.Context, fn func(context.Context, R) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	u.store.mu.Lock()
	defer u.store.mu.Unlock()
	u.store.withinCalls++

	tx := &Tx{orders: cloneRecords(u.store.orders)}
	if err := fn(ctx, u.bind(tx)); err != nil {
		return err
	}

	if err := u.store.failNextCommit; err != nil {
		u.store.failNextCommit = nil
		return err
	}

	u.store.orders = tx.orders
	u.store.outbox = append(u.store.outbox, tx.outbox...)
	return nil
}

type txOrders struct{ tx *Tx }

func (r txOrders) Load(_ context.Context, id orders.OrderID) (orders.Snapshot, dmpfports.Version, error) {
	return load(r.tx.orders, id)
}

func (r txOrders) Save(_ context.Context, id orders.OrderID, state orders.Snapshot, expected dmpfports.Version) error {
	current := dmpfports.Version(0)
	if rec, ok := r.tx.orders[id]; ok {
		current = rec.version
	}
	if current != expected {
		return dmpfports.ErrVersionConflict
	}
	r.tx.orders[id] = record{snapshot: cloneSnapshot(state), version: expected + 1}
	return nil
}

type txOutbox struct{ tx *Tx }

func (o txOutbox) Enqueue(_ context.Context, entry dmpfports.OutboxEntry) error {
	o.tx.outbox = append(o.tx.outbox, entry)
	return nil
}
