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

// recorder collects the order in which the ports are touched, which is how the
// nine-step sequence becomes observable from outside the service.
type recorder struct{ observed []string }

func (r *recorder) record(step string) { r.observed = append(r.observed, step) }

// harness wires the service over the in-memory realization. The bind lives in
// the test because the provider cannot know the resource type; the counters
// live here because bind hands out fresh wrappers on every transaction, and
// per-wrapper counters would hide the repetition UOW-09 forbids.
type harness struct {
	store   *memory.Store
	service application.Service
	rec     *recorder

	binds    int
	loads    int
	saves    int
	enqueues int

	// seedCalls discounts the transactions the fixture itself opened, so a test
	// asserts how many the use case opened, which is what UOW-01 is about.
	seedCalls int
}

// serviceWithinCalls is the number of transactions opened by the service under
// test, excluding the ones seed() opened to arrange the fixture.
func (h *harness) serviceWithinCalls() int {
	return h.store.WithinCalls() - h.seedCalls
}

// serviceCommits is the number of commits that actually installed state during
// the service call. It is asserted apart from serviceWithinCalls because the
// recording wrapper can only observe that Within returned nil, which a
// realization that rolled back would also do.
func (h *harness) serviceCommits() int {
	return h.store.Commits() - h.seedCalls
}

type recordingClock struct {
	inner ports.Clock
	rec   *recorder
}

func (c recordingClock) Now() ports.Instant {
	c.rec.record("clock.Now")
	return c.inner.Now()
}

type recordingIDs struct {
	inner ports.IDGenerator
	rec   *recorder
}

func (g recordingIDs) NewMessageID() ports.MessageID {
	g.rec.record("ids.NewMessageID")
	return g.inner.NewMessageID()
}

// recordingRepository counts through the harness and can inject a Save error,
// which is how a version conflict reaches the use case: the in-memory store
// never produces one on its own, because txMu serializes every transaction.
type recordingRepository struct {
	inner   ports.Repository[domain.OrderID, domain.Snapshot]
	h       *harness
	saveErr error
}

func (r recordingRepository) Load(ctx context.Context, id domain.OrderID) (domain.Snapshot, ports.Version, error) {
	r.h.rec.record("orders.Load")
	r.h.loads++
	return r.inner.Load(ctx, id)
}

func (r recordingRepository) Save(ctx context.Context, id domain.OrderID, state domain.Snapshot, expected ports.Version) error {
	r.h.rec.record("orders.Save")
	r.h.saves++
	if r.saveErr != nil {
		return r.saveErr
	}
	return r.inner.Save(ctx, id, state, expected)
}

type recordingOutbox struct {
	inner ports.Outbox
	h     *harness
}

func (o recordingOutbox) Enqueue(ctx context.Context, entry ports.OutboxEntry) error {
	o.h.rec.record("outbox.Enqueue")
	o.h.enqueues++
	return o.inner.Enqueue(ctx, entry)
}

type recordingUnitOfWork[R any] struct {
	inner ports.UnitOfWork[R]
	rec   *recorder
}

func (u recordingUnitOfWork[R]) Within(ctx context.Context, fn func(context.Context, R) error) error {
	u.rec.record("within")
	err := u.inner.Within(ctx, fn)
	if err == nil {
		u.rec.record("commit")
	}
	return err
}

type option func(*setup)

type setup struct {
	saveErr   error
	authorize usecase.Authorize[application.Operation]
}

func withSaveError(err error) option {
	return func(s *setup) { s.saveErr = err }
}

func withAuthorize(authorize usecase.Authorize[application.Operation]) option {
	return func(s *setup) { s.authorize = authorize }
}

func newHarness(t *testing.T, options ...option) *harness {
	t.Helper()

	h := &harness{store: memory.New(), rec: &recorder{}}
	cfg := &setup{authorize: usecase.AllowAll[application.Operation]()}
	for _, apply := range options {
		apply(cfg)
	}

	bind := func(tx *memory.Tx) application.Resources {
		h.binds++
		return application.Resources{
			Orders: recordingRepository{inner: orderTable.Repository(tx), h: h, saveErr: cfg.saveErr},
			Outbox: recordingOutbox{inner: tx.Outbox(), h: h},
		}
	}

	h.service = application.Service{
		UoW:       recordingUnitOfWork[application.Resources]{inner: memory.NewUnitOfWork(h.store, bind), rec: h.rec},
		Reader:    orderTable.Reader(h.store),
		Clock:     recordingClock{inner: memory.FixedClock{At: occurred}, rec: h.rec},
		IDs:       recordingIDs{inner: &memory.SequenceIDs{Prefix: "m-"}, rec: h.rec},
		Authorize: recordingAuthorize(h.rec, cfg.authorize),
		ItemLimit: itemLimit,
	}
	return h
}

func recordingAuthorize(rec *recorder, inner usecase.Authorize[application.Operation]) usecase.Authorize[application.Operation] {
	return func(ctx context.Context, cmd application.Operation) error {
		rec.record("authorize")
		return inner(ctx, cmd)
	}
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
	h.seedCalls++
	h.rec.observed = nil
}

func openSnapshot(items int) domain.Snapshot {
	built := make([]domain.Item, 0, items)
	for i := range items {
		built = append(built, domain.Item{SKU: domain.SKU(string(rune('A' + i))), Quantity: 1})
	}
	return domain.Snapshot{ID: orderID, Status: domain.Open, ItemLimit: itemLimit, Items: built}
}
