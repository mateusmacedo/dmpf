package serviceskit

import (
	"context"
	"errors"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Fakes are the ports a service receives: the in-memory realization of the
// memory module with every gesture written to the ledger.
// The realization is the same one KIT-04 certifies; the ledger is the
// difference (KIT-03). Fakes is not safe for concurrent use: a service test
// drives it from one goroutine, and the transaction counter relies on that.
type Fakes struct {
	Store  *memory.Store
	Ledger *Ledger
	Steps  *Steps

	// FailRegister fails every command-inbox Register after its step is recorded.
	FailRegister error

	// txs numbers the transactions opened, so every recording port knows which
	// one it belongs to.
	txs int

	// baseline is how many outbox entries the fixture itself committed before
	// the use case under test ran; Decide discounts them (see Baseline).
	baseline int

	seeded int
	binds  int
}

// Baseline marks the current outbox as the fixture's, so a refusal judged
// afterwards is not blamed for entries an accepted setup command produced.
func (f *Fakes) Baseline() { f.baseline = len(f.Store.Entries()) }

// EntriesSinceBaseline counts the outbox entries the use case under test added.
func (f *Fakes) EntriesSinceBaseline() int { return len(f.Store.Entries()) - f.baseline }

func NewFakes() *Fakes {
	return &Fakes{Store: memory.New(), Ledger: &Ledger{}, Steps: &Steps{}}
}

// MarkSeeded discounts one transaction the fixture opened on the store, so
// WithinCalls and Commits count what the use case under test opened (UOW-01).
func (f *Fakes) MarkSeeded() { f.seeded++ }

func (f *Fakes) WithinCalls() int { return f.Store.WithinCalls() - f.seeded }

// Commits counts the commits that installed state, apart from WithinCalls: a
// Within that returned nil is also what a realization that rolled back shows.
func (f *Fakes) Commits() int { return f.Store.Commits() - f.seeded }

// Binds counts every bind of a resource set, so a repetition UOW-09 forbids
// shows even though each transaction hands out fresh wrappers.
func (f *Fakes) Binds() int { return f.binds }

// UnitOfWork wraps memory.NewUnitOfWork: the ports bind receives are the
// recording wrappers of Tx, so every write, enqueue and registration made
// through them lands in the ledger between begin and commit/rollback.
func UnitOfWork[R any](f *Fakes, bind func(Tx) R) ports.UnitOfWork[R] {
	return &recordingUoW[R]{
		fakes: f,
		inner: memory.NewUnitOfWork(f.Store, func(tx *memory.Tx) R {
			f.binds++
			return bind(Tx{inner: tx, fakes: f, id: f.txs})
		}),
	}
}

func (f *Fakes) UnitOfWork(bind func(tx Tx) any) ports.UnitOfWork[any] { return UnitOfWork(f, bind) }

// Tx is the recording view of one open transaction; id is its number in the
// ledger, carried by every port it hands out.
type Tx struct {
	inner *memory.Tx
	fakes *Fakes
	id    int
}

func (t Tx) Outbox() ports.Outbox {
	return recordingOutbox{inner: t.inner.Outbox(), fakes: t.fakes, tx: t.id}
}

func (t Tx) Inbox(consumer string) ports.Inbox {
	return recordingInbox{inner: t.inner.Inbox(consumer), ledger: t.fakes.Ledger, tx: t.id}
}

// CommandInbox is the inbox of a context's commands (IDM-03): Register and the
// Complete of its first reception are recorded as steps.
func (t Tx) CommandInbox(consumer string) ports.Inbox {
	return recordingCommands{inner: t.inner.CommandInbox(consumer), fakes: t.fakes, tx: t.id}
}

// Repository wraps a transactional repository of the open transaction, so a
// use case over any aggregate can be observed; Write is recorded per Save.
func Repository[ID comparable, S any](t Tx, inner ports.Repository[ID, S], name func(ID) string) ports.Repository[ID, S] {
	return recordingRepository[ID, S]{inner: inner, ledger: t.fakes.Ledger, name: name, tx: t.id}
}

// Memory exposes the underlying transaction for aggregates the recording
// wrappers do not know by name.
func (t Tx) Memory() *memory.Tx { return t.inner }

// Publisher is a fake broker: a service that reaches it inside the sequence
// violates UOW-08, and the ledger shows exactly where.
type Publisher struct{ ledger *Ledger }

func (f *Fakes) Publisher() *Publisher { return &Publisher{ledger: f.Ledger} }

func (p *Publisher) Publish(_ context.Context, destination string, _ []byte) error {
	p.ledger.record(0, Publish, destination)
	return nil
}

type recordingUoW[R any] struct {
	fakes *Fakes
	inner ports.UnitOfWork[R]
}

func (u *recordingUoW[R]) Within(ctx context.Context, fn func(context.Context, R) error) error {
	u.fakes.txs++
	id := u.fakes.txs
	u.fakes.Ledger.record(id, Begin, "")
	u.fakes.Steps.Record("within")
	err := u.inner.Within(ctx, fn)
	if err != nil {
		u.fakes.Ledger.record(id, Rollback, err.Error())
		return err
	}
	u.fakes.Ledger.record(id, Commit, "")
	u.fakes.Steps.Record("commit")
	return nil
}

type recordingRepository[ID comparable, S any] struct {
	inner  ports.Repository[ID, S]
	ledger *Ledger
	name   func(ID) string
	tx     int
}

func (r recordingRepository[ID, S]) Load(ctx context.Context, id ID) (S, ports.Version, error) {
	return r.inner.Load(ctx, id)
}

func (r recordingRepository[ID, S]) Save(ctx context.Context, id ID, state S, expected ports.Version) error {
	if err := r.inner.Save(ctx, id, state, expected); err != nil {
		return err
	}
	r.ledger.record(r.tx, Write, r.name(id))
	return nil
}

type recordingOutbox struct {
	inner ports.Outbox
	fakes *Fakes
	tx    int
}

func (o recordingOutbox) Enqueue(ctx context.Context, entry ports.OutboxEntry) error {
	o.fakes.Steps.Record("outbox.Enqueue")
	if err := o.inner.Enqueue(ctx, entry); err != nil {
		return err
	}
	o.fakes.Ledger.record(o.tx, Enqueue, string(entry.MessageID))
	return nil
}

type recordingInbox struct {
	inner  ports.Inbox
	ledger *Ledger
	tx     int
}

func (i recordingInbox) Register(ctx context.Context, r ports.Receipt) (ports.Reception, error) {
	reception, err := i.inner.Register(ctx, r)
	if err != nil {
		return reception, err
	}
	i.ledger.record(i.tx, Register, string(r.MessageID))
	return reception, nil
}

type recordingCommands struct {
	inner ports.Inbox
	fakes *Fakes
	tx    int
}

var errFirstReception = errors.New("serviceskit: first reception")

func (c recordingCommands) Register(ctx context.Context, r ports.Receipt) (ports.Reception, error) {
	c.fakes.Steps.Record("commands.Register")
	if c.fakes.FailRegister != nil {
		return ports.Reception{}, c.fakes.FailRegister
	}
	reception, err := c.inner.Register(ctx, r)
	if err != nil {
		return reception, err
	}
	c.fakes.Ledger.record(c.tx, Register, string(r.MessageID))

	// WHY: Match is the only way into a Reception; a sentinel stops it at the
	// first branch, so the pending entry is wrapped and nothing is completed here.
	var pending ports.Pending
	capture := func(p ports.Pending) error { pending = p; return errFirstReception }
	keep := func() error { return nil }
	if err := reception.Match(capture, keep, keep, keep); !errors.Is(err, errFirstReception) {
		return reception, err
	}
	return ports.FirstReception(recordingPending{inner: pending, steps: c.fakes.Steps}), nil
}

type recordingPending struct {
	inner ports.Pending
	steps *Steps
}

func (p recordingPending) Complete(ctx context.Context, c ports.Completion) error {
	p.steps.Record("commands.Complete")
	return p.inner.Complete(ctx, c)
}

func (p recordingPending) Completed() bool { return p.inner.Completed() }
