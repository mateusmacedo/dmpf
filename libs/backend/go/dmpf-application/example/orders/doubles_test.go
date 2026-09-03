package ordersapp_test

import (
	"context"
	"testing"

	dmpfapplication "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/memory"
	ordersapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/orders"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

const (
	orderID   = orders.OrderID("P-100")
	occurred  = dmpfports.Instant(1_755_432_000_000_000_000)
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
	service ordersapp.Service
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

type recordingClock struct {
	inner dmpfports.Clock
	rec   *recorder
}

func (c recordingClock) Now() dmpfports.Instant {
	c.rec.record("clock.Now")
	return c.inner.Now()
}

type recordingIDs struct {
	inner dmpfports.IDGenerator
	rec   *recorder
}

func (g recordingIDs) NewMessageID() dmpfports.MessageID {
	g.rec.record("ids.NewMessageID")
	return g.inner.NewMessageID()
}

// recordingRepository counts through the harness and can inject a Save error,
// which is how a version conflict reaches the use case: the in-memory store
// never produces one on its own, because one mutex serializes transactions.
type recordingRepository struct {
	inner   dmpfports.Repository[orders.OrderID, orders.Snapshot]
	h       *harness
	saveErr error
}

func (r recordingRepository) Load(ctx context.Context, id orders.OrderID) (orders.Snapshot, dmpfports.Version, error) {
	r.h.rec.record("orders.Load")
	r.h.loads++
	return r.inner.Load(ctx, id)
}

func (r recordingRepository) Save(ctx context.Context, id orders.OrderID, state orders.Snapshot, expected dmpfports.Version) error {
	r.h.rec.record("orders.Save")
	r.h.saves++
	if r.saveErr != nil {
		return r.saveErr
	}
	return r.inner.Save(ctx, id, state, expected)
}

type recordingOutbox struct {
	inner dmpfports.Outbox
	h     *harness
}

func (o recordingOutbox) Enqueue(ctx context.Context, entry dmpfports.OutboxEntry) error {
	o.h.rec.record("outbox.Enqueue")
	o.h.enqueues++
	return o.inner.Enqueue(ctx, entry)
}

type recordingUnitOfWork[R any] struct {
	inner dmpfports.UnitOfWork[R]
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
	authorize dmpfapplication.AuthorizeFunc[ordersapp.Command]
}

func withSaveError(err error) option {
	return func(s *setup) { s.saveErr = err }
}

func withAuthorize(authorize dmpfapplication.AuthorizeFunc[ordersapp.Command]) option {
	return func(s *setup) { s.authorize = authorize }
}

func newHarness(t *testing.T, options ...option) *harness {
	t.Helper()

	h := &harness{store: memory.New(), rec: &recorder{}}
	cfg := &setup{authorize: dmpfapplication.AllowAll[ordersapp.Command]()}
	for _, apply := range options {
		apply(cfg)
	}

	bind := func(tx *memory.Tx) ordersapp.Resources {
		h.binds++
		return ordersapp.Resources{
			Orders: recordingRepository{inner: tx.Orders(), h: h, saveErr: cfg.saveErr},
			Outbox: recordingOutbox{inner: tx.Outbox(), h: h},
		}
	}

	h.service = ordersapp.Service{
		UoW:       recordingUnitOfWork[ordersapp.Resources]{inner: memory.NewUnitOfWork(h.store, bind), rec: h.rec},
		Reader:    h.store.Reader(),
		Clock:     recordingClock{inner: memory.FixedClock{At: occurred}, rec: h.rec},
		IDs:       recordingIDs{inner: &memory.SequenceIDs{Prefix: "m-"}, rec: h.rec},
		Authorize: recordingAuthorize(h.rec, cfg.authorize),
		ItemLimit: itemLimit,
	}
	return h
}

func recordingAuthorize(rec *recorder, inner dmpfapplication.AuthorizeFunc[ordersapp.Command]) dmpfapplication.AuthorizeFunc[ordersapp.Command] {
	return func(ctx context.Context, cmd ordersapp.Command) error {
		rec.record("authorize")
		return inner(ctx, cmd)
	}
}

// seed puts an aggregate in the store through a plain transaction, so the
// service under test starts from a known version without being the writer. It
// clears the observations and discounts its own transaction.
func (h *harness) seed(t *testing.T, snapshot orders.Snapshot, expected dmpfports.Version) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, func(tx *memory.Tx) ordersapp.Resources {
		return ordersapp.Resources{Orders: tx.Orders(), Outbox: tx.Outbox()}
	})
	err := uow.Within(context.Background(), func(ctx context.Context, res ordersapp.Resources) error {
		return res.Orders.Save(ctx, snapshot.ID, snapshot, expected)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	h.seedCalls++
	h.rec.observed = nil
}

func openSnapshot(items int) orders.Snapshot {
	built := make([]orders.Item, 0, items)
	for i := range items {
		built = append(built, orders.Item{SKU: orders.SKU(string(rune('A' + i))), Quantity: 1})
	}
	return orders.Snapshot{ID: orderID, Status: orders.Open, ItemLimit: itemLimit, Items: built}
}
