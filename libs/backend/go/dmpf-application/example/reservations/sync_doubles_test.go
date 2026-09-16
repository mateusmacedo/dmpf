package reservationsapp_test

import (
	"context"
	"testing"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/memory"
	reservationsapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/reservations"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

const (
	syncOrder    = reservations.OrderID("P-100")
	syncOccurred = dmpfports.Instant(1_755_432_000_000_000_000)
)

type syncRecorder struct{ observed []string }

func (r *syncRecorder) record(step string) { r.observed = append(r.observed, step) }

type syncHarness struct {
	store   *memory.Store
	service reservationsapp.Service
	rec     *syncRecorder

	binds    int
	saves    int
	enqueues int

	seedCalls int
}

func (h *syncHarness) serviceWithinCalls() int { return h.store.WithinCalls() - h.seedCalls }

func (h *syncHarness) serviceCommits() int { return h.store.Commits() - h.seedCalls }

type syncClock struct {
	inner dmpfports.Clock
	rec   *syncRecorder
}

func (c syncClock) Now() dmpfports.Instant {
	c.rec.record("clock.Now")
	return c.inner.Now()
}

type syncIDs struct {
	inner dmpfports.IDGenerator
	rec   *syncRecorder
}

func (g syncIDs) NewMessageID() dmpfports.MessageID {
	g.rec.record("ids.NewMessageID")
	return g.inner.NewMessageID()
}

type syncRepository struct {
	inner   dmpfports.Repository[reservations.OrderID, reservations.Snapshot]
	h       *syncHarness
	saveErr error
}

func (r syncRepository) Load(ctx context.Context, id reservations.OrderID) (reservations.Snapshot, dmpfports.Version, error) {
	r.h.rec.record("reservations.Load")
	return r.inner.Load(ctx, id)
}

func (r syncRepository) Save(ctx context.Context, id reservations.OrderID, state reservations.Snapshot, expected dmpfports.Version) error {
	r.h.rec.record("reservations.Save")
	r.h.saves++
	if r.saveErr != nil {
		return r.saveErr
	}
	return r.inner.Save(ctx, id, state, expected)
}

type syncOutbox struct {
	inner dmpfports.Outbox
	h     *syncHarness
}

func (o syncOutbox) Enqueue(ctx context.Context, entry dmpfports.OutboxEntry) error {
	o.h.rec.record("outbox.Enqueue")
	o.h.enqueues++
	return o.inner.Enqueue(ctx, entry)
}

type syncUnitOfWork struct {
	inner dmpfports.UnitOfWork[reservationsapp.Resources]
	rec   *syncRecorder
}

func (u syncUnitOfWork) Within(ctx context.Context, fn func(context.Context, reservationsapp.Resources) error) error {
	u.rec.record("within")
	err := u.inner.Within(ctx, fn)
	if err == nil {
		u.rec.record("commit")
	}
	return err
}

type syncOption func(*syncSetup)

type syncSetup struct {
	saveErr   error
	authorize dmpfapplication.AuthorizeFunc[reservationsapp.Command]
}

func withSyncSaveError(err error) syncOption {
	return func(s *syncSetup) { s.saveErr = err }
}

func withSyncAuthorize(authorize dmpfapplication.AuthorizeFunc[reservationsapp.Command]) syncOption {
	return func(s *syncSetup) { s.authorize = authorize }
}

func newSyncHarness(t *testing.T, options ...syncOption) *syncHarness {
	t.Helper()

	h := &syncHarness{store: memory.New(), rec: &syncRecorder{}}
	cfg := &syncSetup{authorize: dmpfapplication.AllowAll[reservationsapp.Command]()}
	for _, apply := range options {
		apply(cfg)
	}

	bindSync := func(tx *memory.Tx) reservationsapp.Resources {
		h.binds++
		return reservationsapp.Resources{
			Inbox:        tx.Inbox(consumer),
			Reservations: syncRepository{inner: tx.Reservations(), h: h, saveErr: cfg.saveErr},
			Outbox:       syncOutbox{inner: tx.Outbox(), h: h},
		}
	}

	authorize := cfg.authorize
	h.service = reservationsapp.Service{
		UoW:    syncUnitOfWork{inner: memory.NewUnitOfWork(h.store, bindSync), rec: h.rec},
		Reader: h.store.ReservationsReader(),
		Clock:  syncClock{inner: memory.FixedClock{At: syncOccurred}, rec: h.rec},
		IDs:    syncIDs{inner: &memory.SequenceIDs{Prefix: "m-"}, rec: h.rec},
		Authorize: func(ctx context.Context, cmd reservationsapp.Command) error {
			h.rec.record("authorize")
			return authorize(ctx, cmd)
		},
		Consumer: consumer,
	}
	return h
}

func (h *syncHarness) seed(t *testing.T, snapshot reservations.Snapshot, expected dmpfports.Version) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, bind)
	err := uow.Within(context.Background(), func(ctx context.Context, res reservationsapp.Resources) error {
		return res.Reservations.Save(ctx, snapshot.Order, snapshot, expected)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	h.seedCalls++
	h.rec.observed = nil
}

func confirmedSnapshot(items int) reservations.Snapshot {
	return reservations.Snapshot{Order: syncOrder, Items: items, Status: reservations.Confirmed}
}

func canceledSnapshot() reservations.Snapshot {
	return reservations.Snapshot{Order: syncOrder, Status: reservations.Canceled}
}
