package memory

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Tx is the working copy of one transaction: writes land here and reach the
// store only at commit.
type Tx struct {
	orders       map[orders.OrderID]record
	reservations map[reservations.OrderID]reservationRecord
	inbox        map[inboxKey]inboxRow
	outbox       []ports.OutboxEntry
}

// Orders is the transactional repository of the open transaction.
func (t *Tx) Orders() ports.Repository[orders.OrderID, orders.Snapshot] {
	return txOrders{tx: t}
}

// Reservations is the transactional repository of the open transaction,
// mirroring Orders for the reservations aggregate.
func (t *Tx) Reservations() ports.Repository[reservations.OrderID, reservations.Snapshot] {
	return txReservations{tx: t}
}

// Inbox is the transactional Inbox bound to consumer, the deduplication
// boundary of the open transaction.
func (t *Tx) Inbox(consumer string) ports.Inbox {
	return txInbox{tx: t, consumer: consumer}
}

// Outbox is the transactional outbox of the open transaction.
func (t *Tx) Outbox() ports.Outbox {
	return txOutbox{tx: t}
}

// NewUnitOfWork binds an open transaction to the resource set R that a use case
// declares. bind is written by the composition root, never here: provider →
// application is a forbidden cell, so this package cannot know R's shape.
func NewUnitOfWork[R any](store *Store, bind func(tx *Tx) R) ports.UnitOfWork[R] {
	return unitOfWork[R]{store: store, bind: bind}
}

type unitOfWork[R any] struct {
	store *Store
	bind  func(tx *Tx) R
}

// Within holds txMu for the whole callback, which serializes transactions and
// is why this realization proves atomicity but not isolation. It holds dataMu
// only to snapshot the state and to commit, so a Reader() call from inside the
// callback reads instead of deadlocking. A panic in fn unwinds without touching
// the store, so the discarded Tx is the rollback (ERR-22).
func (u unitOfWork[R]) Within(ctx context.Context, fn func(context.Context, R) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	u.store.txMu.Lock()
	defer u.store.txMu.Unlock()

	// Revalidado depois da espera pelo mutex: quem ficou na fila pode ter tido o
	// contexto cancelado enquanto esperava, e é aqui que a transação abre.
	if err := ctx.Err(); err != nil {
		return err
	}

	tx := u.open()
	if err := fn(ctx, u.bind(tx)); err != nil {
		return err
	}
	return u.commit(tx)
}

func (u unitOfWork[R]) open() *Tx {
	u.store.dataMu.Lock()
	defer u.store.dataMu.Unlock()
	u.store.withinCalls++
	return &Tx{
		orders:       cloneRecords(u.store.orders),
		reservations: cloneReservationRecords(u.store.reservations),
		inbox:        cloneInboxRows(u.store.inbox),
	}
}

func (u unitOfWork[R]) commit(tx *Tx) error {
	u.store.dataMu.Lock()
	defer u.store.dataMu.Unlock()

	if err := u.store.failNextCommit; err != nil {
		u.store.failNextCommit = nil
		return err
	}

	// Clona em vez de instalar o mapa da Tx: uma porta transacional que escape do
	// callback continua escrevendo na cópia descartada, e não no estado do Store,
	// fora de qualquer transação e sem o dataMu.
	u.store.orders = cloneRecords(tx.orders)
	u.store.reservations = cloneReservationRecords(tx.reservations)
	u.store.inbox = cloneInboxRows(tx.inbox)
	u.store.outbox = append(u.store.outbox, tx.outbox...)
	u.store.commits++
	return nil
}

type txOrders struct{ tx *Tx }

func (r txOrders) Load(_ context.Context, id orders.OrderID) (orders.Snapshot, ports.Version, error) {
	return load(r.tx.orders, id)
}

func (r txOrders) Save(_ context.Context, id orders.OrderID, state orders.Snapshot, expected ports.Version) error {
	current := ports.Version(0)
	if rec, ok := r.tx.orders[id]; ok {
		current = rec.version
	}
	if current != expected {
		return ports.ErrVersionConflict
	}
	r.tx.orders[id] = record{snapshot: cloneSnapshot(state), version: expected + 1}
	return nil
}

type txReservations struct{ tx *Tx }

func (r txReservations) Load(_ context.Context, id reservations.OrderID) (reservations.Snapshot, ports.Version, error) {
	return loadReservation(r.tx.reservations, id)
}

func (r txReservations) Save(_ context.Context, id reservations.OrderID, state reservations.Snapshot, expected ports.Version) error {
	current := ports.Version(0)
	if rec, ok := r.tx.reservations[id]; ok {
		current = rec.version
	}
	if current != expected {
		return ports.ErrVersionConflict
	}
	r.tx.reservations[id] = reservationRecord{snapshot: state, version: expected + 1}
	return nil
}

type txOutbox struct{ tx *Tx }

func (o txOutbox) Enqueue(_ context.Context, entry ports.OutboxEntry) error {
	o.tx.outbox = append(o.tx.outbox, entry)
	return nil
}
