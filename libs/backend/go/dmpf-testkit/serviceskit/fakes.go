package serviceskit

import (
	"context"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/memory"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// Fakes are the ports a service receives: the in-memory realization of
// dmpf-application/example/memory with every gesture written to the ledger.
// The realization is the same one KIT-04 certifies; the ledger is the
// difference (KIT-03).
type Fakes struct {
	Store  *memory.Store
	Ledger *Ledger

	// baseline is how many outbox entries the fixture itself committed before
	// the use case under test ran; Decide discounts them (see Baseline).
	baseline int
}

// Baseline marks the current outbox as the fixture's, so a refusal judged
// afterwards is not blamed for entries an accepted setup command produced.
func (f *Fakes) Baseline() { f.baseline = len(f.Store.Entries()) }

// EntriesSinceBaseline counts the outbox entries the use case under test added.
func (f *Fakes) EntriesSinceBaseline() int { return len(f.Store.Entries()) - f.baseline }

func NewFakes() *Fakes {
	return &Fakes{Store: memory.New(), Ledger: &Ledger{}}
}

// UnitOfWork wraps memory.NewUnitOfWork: the ports bind receives are the
// recording wrappers of Tx, so every write, enqueue and registration made
// through them lands in the ledger between begin and commit/rollback.
func (f *Fakes) UnitOfWork(bind func(tx Tx) any) dmpfports.UnitOfWork[any] {
	return recordingUoW{
		ledger: f.Ledger,
		inner: memory.NewUnitOfWork(f.Store, func(tx *memory.Tx) any {
			return bind(Tx{inner: tx, ledger: f.Ledger})
		}),
	}
}

// Tx is the recording view of one open transaction.
type Tx struct {
	inner  *memory.Tx
	ledger *Ledger
}

func (t Tx) Outbox() dmpfports.Outbox {
	return recordingOutbox{inner: t.inner.Outbox(), ledger: t.ledger}
}

func (t Tx) Inbox(consumer string) dmpfports.Inbox {
	return recordingInbox{inner: t.inner.Inbox(consumer), ledger: t.ledger}
}

// Repository wraps a transactional repository of the open transaction, so a
// use case over any aggregate can be observed; Write is recorded per Save.
func Repository[ID comparable, S any](t Tx, inner dmpfports.Repository[ID, S], name func(ID) string) dmpfports.Repository[ID, S] {
	return recordingRepository[ID, S]{inner: inner, ledger: t.ledger, name: name}
}

// Memory exposes the underlying transaction for aggregates the recording
// wrappers do not know by name.
func (t Tx) Memory() *memory.Tx { return t.inner }

// Publisher is a fake broker: a service that reaches it inside the sequence
// violates UOW-08, and the ledger shows exactly where.
type Publisher struct{ ledger *Ledger }

func (f *Fakes) Publisher() *Publisher { return &Publisher{ledger: f.Ledger} }

func (p *Publisher) Publish(_ context.Context, destination string, _ []byte) error {
	p.ledger.record(Publish, destination)
	return nil
}

type recordingUoW struct {
	ledger *Ledger
	inner  dmpfports.UnitOfWork[any]
}

func (u recordingUoW) Within(ctx context.Context, fn func(context.Context, any) error) error {
	u.ledger.record(Begin, "")
	err := u.inner.Within(ctx, fn)
	if err != nil {
		u.ledger.record(Rollback, err.Error())
		return err
	}
	u.ledger.record(Commit, "")
	return nil
}

type recordingRepository[ID comparable, S any] struct {
	inner  dmpfports.Repository[ID, S]
	ledger *Ledger
	name   func(ID) string
}

func (r recordingRepository[ID, S]) Load(ctx context.Context, id ID) (S, dmpfports.Version, error) {
	return r.inner.Load(ctx, id)
}

func (r recordingRepository[ID, S]) Save(ctx context.Context, id ID, state S, expected dmpfports.Version) error {
	if err := r.inner.Save(ctx, id, state, expected); err != nil {
		return err
	}
	r.ledger.record(Write, r.name(id))
	return nil
}

type recordingOutbox struct {
	inner  dmpfports.Outbox
	ledger *Ledger
}

func (o recordingOutbox) Enqueue(ctx context.Context, entry dmpfports.OutboxEntry) error {
	if err := o.inner.Enqueue(ctx, entry); err != nil {
		return err
	}
	o.ledger.record(Enqueue, string(entry.MessageID))
	return nil
}

type recordingInbox struct {
	inner  dmpfports.Inbox
	ledger *Ledger
}

func (i recordingInbox) Register(ctx context.Context, r dmpfports.Receipt) (dmpfports.Reception, error) {
	reception, err := i.inner.Register(ctx, r)
	if err != nil {
		return reception, err
	}
	i.ledger.record(Register, string(r.MessageID))
	return reception, nil
}
