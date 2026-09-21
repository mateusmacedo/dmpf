package application_test

import (
	"context"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

const (
	syncOrder    = domain.OrderID("P-100")
	syncOccurred = ports.Instant(1_755_432_000_000_000_000)
)

type syncRecorder struct{ observed []string }

func (r *syncRecorder) record(step string) { r.observed = append(r.observed, step) }

type syncHarness struct {
	store   *memory.Store
	service application.Service
	rec     *syncRecorder

	binds    int
	saves    int
	enqueues int

	seedCalls int
}

func (h *syncHarness) serviceWithinCalls() int { return h.store.WithinCalls() - h.seedCalls }

func (h *syncHarness) serviceCommits() int { return h.store.Commits() - h.seedCalls }

type syncClock struct {
	inner ports.Clock
	rec   *syncRecorder
}

func (c syncClock) Now() ports.Instant {
	c.rec.record("clock.Now")
	return c.inner.Now()
}

type syncIDs struct {
	inner ports.IDGenerator
	rec   *syncRecorder
}

func (g syncIDs) NewMessageID() ports.MessageID {
	g.rec.record("ids.NewMessageID")
	return g.inner.NewMessageID()
}

type syncRepository struct {
	inner   ports.Repository[domain.OrderID, domain.Snapshot]
	h       *syncHarness
	saveErr error
}

func (r syncRepository) Load(ctx context.Context, id domain.OrderID) (domain.Snapshot, ports.Version, error) {
	r.h.rec.record("domain.Load")
	return r.inner.Load(ctx, id)
}

func (r syncRepository) Save(ctx context.Context, id domain.OrderID, state domain.Snapshot, expected ports.Version) error {
	r.h.rec.record("domain.Save")
	r.h.saves++
	if r.saveErr != nil {
		return r.saveErr
	}
	return r.inner.Save(ctx, id, state, expected)
}

type syncOutbox struct {
	inner ports.Outbox
	h     *syncHarness
}

func (o syncOutbox) Enqueue(ctx context.Context, entry ports.OutboxEntry) error {
	o.h.rec.record("outbox.Enqueue")
	o.h.enqueues++
	return o.inner.Enqueue(ctx, entry)
}

type syncUnitOfWork struct {
	inner ports.UnitOfWork[application.Resources]
	rec   *syncRecorder
}

func (u syncUnitOfWork) Within(ctx context.Context, fn func(context.Context, application.Resources) error) error {
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
	authorize usecase.AuthorizeWithContext[application.Operation]
}

func withSyncSaveError(err error) syncOption {
	return func(s *syncSetup) { s.saveErr = err }
}

func withSyncAuthorize(authorize usecase.AuthorizeWithContext[application.Operation]) syncOption {
	return func(s *syncSetup) { s.authorize = authorize }
}

func newSyncHarness(t *testing.T, options ...syncOption) *syncHarness {
	t.Helper()

	h := &syncHarness{store: memory.New(), rec: &syncRecorder{}}
	cfg := &syncSetup{authorize: usecase.AllowAllWithContext[application.Operation]()}
	for _, apply := range options {
		apply(cfg)
	}

	bindSync := func(tx *memory.Tx) application.Resources {
		h.binds++
		return application.Resources{
			Inbox:        tx.Inbox(consumer),
			Reservations: syncRepository{inner: reservationsTable.Repository(tx), h: h, saveErr: cfg.saveErr},
			Outbox:       syncOutbox{inner: tx.Outbox(), h: h},
		}
	}

	authorize := cfg.authorize
	h.service = application.Service{
		UoW:    syncUnitOfWork{inner: memory.NewUnitOfWork(h.store, bindSync), rec: h.rec},
		Reader: reservationsTable.Reader(h.store),
		Clock:  syncClock{inner: memory.FixedClock{At: syncOccurred}, rec: h.rec},
		IDs:    syncIDs{inner: &memory.SequenceIDs{Prefix: "m-"}, rec: h.rec},
		Authorize: func(ctx context.Context, execution ports.ExecutionContext, cmd application.Operation) error {
			h.rec.record("authorize")
			return authorize(ctx, execution, cmd)
		},
		Consumer: consumer,
	}
	return h
}

func (h *syncHarness) seed(t *testing.T, snapshot domain.Snapshot, expected ports.Version) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, bind)
	err := uow.Within(context.Background(), func(ctx context.Context, res application.Resources) error {
		return res.Reservations.Save(ctx, snapshot.Order, snapshot, expected)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	h.seedCalls++
	h.rec.observed = nil
}

func confirmedSnapshot(items int) domain.Snapshot {
	return domain.Snapshot{Order: syncOrder, Items: items, Status: domain.Confirmed}
}

func canceledSnapshot() domain.Snapshot {
	return domain.Snapshot{Order: syncOrder, Status: domain.Canceled}
}
