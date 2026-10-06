package application_test

import (
	"context"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
)

var orderTable = memory.Table[domain.OrderID, domain.Snapshot]{
	Name:  "orders",
	Clone: func(s domain.Snapshot) domain.Snapshot { s.Items = slices.Clone(s.Items); return s },
}

const (
	orderID   = domain.OrderID("P-100")
	occurred  = ports.Instant(1_755_432_000_000_000_000)
	itemLimit = 3
)

type harness struct {
	fakes   *serviceskit.Fakes
	store   *memory.Store
	service application.Service
	rec     *serviceskit.Steps

	loads int
	saves int
}

func (h *harness) enqueues() int { return h.fakes.Ledger.Count(serviceskit.Enqueue) }

type recordingRepository struct {
	inner ports.Repository[domain.OrderID, domain.Snapshot]
	h     *harness
}

func (r recordingRepository) Load(ctx context.Context, id domain.OrderID) (domain.Snapshot, ports.Version, error) {
	r.h.rec.Record("orders.Load")
	r.h.loads++
	return r.inner.Load(ctx, id)
}

func (r recordingRepository) Save(ctx context.Context, id domain.OrderID, state domain.Snapshot, expected ports.Version) error {
	r.h.rec.Record("orders.Save")
	r.h.saves++
	return r.inner.Save(ctx, id, state, expected)
}

type option func(*setup)

type setup struct {
	faults      serviceskit.Faults
	registerErr error
	authorize   usecase.Authorize[application.Operation]
}

func withRegisterError(err error) option {
	return func(s *setup) { s.registerErr = err }
}

func withLoadError(err error) option {
	return func(s *setup) { s.faults.Load = err }
}

func withSaveError(err error) option {
	return func(s *setup) { s.faults.Save = err }
}

func withEnqueueError(err error) option {
	return func(s *setup) { s.faults.Enqueue = err }
}

func withAuthorize(authorize usecase.Authorize[application.Operation]) option {
	return func(s *setup) { s.authorize = authorize }
}

func newHarness(t *testing.T, options ...option) *harness {
	t.Helper()

	fakes := serviceskit.NewFakes()
	h := &harness{fakes: fakes, store: fakes.Store, rec: fakes.Steps}
	cfg := &setup{authorize: usecase.AllowAll[application.Operation]()}
	for _, apply := range options {
		apply(cfg)
	}
	fakes.FailRegister = cfg.registerErr

	bind := func(tx serviceskit.Tx) application.Resources {
		return application.Resources{
			Orders:   recordingRepository{inner: serviceskit.FaultyRepository(orderTable.Repository(tx.Memory()), cfg.faults), h: h},
			Outbox:   serviceskit.FaultyOutbox(tx.Outbox(), cfg.faults),
			Commands: tx.CommandInbox(application.CommandConsumer),
		}
	}

	h.service = application.Service{
		UoW:       serviceskit.UnitOfWork(fakes, bind),
		Reader:    orderTable.Reader(h.store),
		Clock:     fakes.Clock(memory.FixedClock{At: occurred}),
		IDs:       fakes.IDs(&memory.SequenceIDs{Prefix: "m-"}),
		Authorize: serviceskit.Authorize(fakes, cfg.authorize),
		ItemLimit: itemLimit,
		Idempotency: usecase.IdempotencyPolicy{
			Wait: 1_000_000_000, Retention: 86_400_000_000_000, Digest: serviceskit.FoldDigest,
		},
	}
	return h
}

// seed puts an aggregate in the store through a plain transaction, so the
// service under test starts from a known version without being the writer. It
// clears the observations and discounts its own transaction.
func (h *harness) seed(t *testing.T, snapshot domain.Snapshot, expected ports.Version) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, func(tx *memory.Tx) application.Resources {
		return application.Resources{Orders: orderTable.Repository(tx), Outbox: tx.Outbox()}
	})
	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res application.Resources) error {
		return res.Orders.Save(ctx, snapshot.ID, snapshot, expected)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	h.fakes.MarkSeeded()
	h.rec.Reset()
}

func openSnapshot(items int) domain.Snapshot {
	built := make([]domain.Item, 0, items)
	for i := range items {
		built = append(built, domain.Item{SKU: domain.SKU(string(rune('A' + i))), Quantity: 1})
	}
	return domain.Snapshot{ID: orderID, Status: domain.Open, ItemLimit: itemLimit, Items: built}
}
