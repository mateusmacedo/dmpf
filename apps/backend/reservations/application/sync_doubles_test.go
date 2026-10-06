package application_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
)

const (
	syncOrder    = domain.OrderID("P-100")
	syncOccurred = ports.Instant(1_755_432_000_000_000_000)
)

type syncHarness struct {
	fakes   *serviceskit.Fakes
	store   *memory.Store
	service application.Service
	rec     *serviceskit.Steps

	saves int
}

func (h *syncHarness) enqueues() int { return h.fakes.Ledger.Count(serviceskit.Enqueue) }

type syncRepository struct {
	inner ports.Repository[domain.OrderID, domain.Snapshot]
	h     *syncHarness
}

func (r syncRepository) Load(ctx context.Context, id domain.OrderID) (domain.Snapshot, ports.Version, error) {
	r.h.rec.Record("domain.Load")
	return r.inner.Load(ctx, id)
}

func (r syncRepository) Save(ctx context.Context, id domain.OrderID, state domain.Snapshot, expected ports.Version) error {
	r.h.rec.Record("domain.Save")
	r.h.saves++
	return r.inner.Save(ctx, id, state, expected)
}

type syncOption func(*syncSetup)

type syncSetup struct {
	faults      serviceskit.Faults
	registerErr error
	authorize   usecase.Authorize[application.Operation]
}

func withSyncRegisterError(err error) syncOption {
	return func(s *syncSetup) { s.registerErr = err }
}

func withSyncLoadError(err error) syncOption {
	return func(s *syncSetup) { s.faults.Load = err }
}

func withSyncSaveError(err error) syncOption {
	return func(s *syncSetup) { s.faults.Save = err }
}

func withSyncEnqueueError(err error) syncOption {
	return func(s *syncSetup) { s.faults.Enqueue = err }
}

func withSyncAuthorize(authorize usecase.Authorize[application.Operation]) syncOption {
	return func(s *syncSetup) { s.authorize = authorize }
}

func newSyncHarness(t *testing.T, options ...syncOption) *syncHarness {
	t.Helper()

	fakes := serviceskit.NewFakes()
	h := &syncHarness{fakes: fakes, store: fakes.Store, rec: fakes.Steps}
	cfg := &syncSetup{authorize: usecase.AllowAll[application.Operation]()}
	for _, apply := range options {
		apply(cfg)
	}
	fakes.FailRegister = cfg.registerErr

	bindSync := func(tx serviceskit.Tx) application.Resources {
		return application.Resources{
			Inbox:        tx.Memory().Inbox(consumer),
			Reservations: syncRepository{inner: serviceskit.FaultyRepository(reservationTable.Repository(tx.Memory()), cfg.faults), h: h},
			Outbox:       serviceskit.FaultyOutbox(tx.Outbox(), cfg.faults),
			Commands:     tx.CommandInbox(application.CommandConsumer),
		}
	}

	h.service = application.Service{
		UoW:       serviceskit.UnitOfWork(fakes, bindSync),
		Reader:    reservationTable.Reader(h.store),
		Clock:     fakes.Clock(memory.FixedClock{At: syncOccurred}),
		IDs:       fakes.IDs(&memory.SequenceIDs{Prefix: "m-"}),
		Authorize: serviceskit.Authorize(fakes, cfg.authorize),
		Consumer:  consumer,
		Idempotency: usecase.IdempotencyPolicy{
			Wait: 1_000_000_000, Retention: 86_400_000_000_000, Digest: serviceskit.FoldDigest,
		},
	}
	return h
}

func (h *syncHarness) seed(t *testing.T, snapshot domain.Snapshot, expected ports.Version) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, bind)
	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res application.Resources) error {
		return res.Reservations.Save(ctx, snapshot.Order, snapshot, expected)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	h.fakes.MarkSeeded()
	h.rec.Reset()
}

func confirmedSnapshot(items int) domain.Snapshot {
	return domain.Snapshot{Order: syncOrder, Items: items, Status: domain.Confirmed}
}

func canceledSnapshot() domain.Snapshot {
	return domain.Snapshot{Order: syncOrder, Status: domain.Cancelled}
}
