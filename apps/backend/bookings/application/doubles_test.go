package application_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
)

const (
	testBookingID  = domain.BookingID("B-100")
	testResourceID = domain.ResourceID("R-200")
	testResCode    = domain.ResourceCode("room-101")
)

var testOccurred = ports.Instant(1_755_432_000_000_000_000)

var (
	bookingTable  = memory.Table[domain.BookingID, domain.BookingSnapshot]{Name: "bookings"}
	resourceTable = memory.Table[domain.ResourceCode, domain.ResourceSnapshot]{Name: "resources"}
)

type harness struct {
	fakes   *serviceskit.Fakes
	store   *memory.Store
	rec     *serviceskit.Steps
	service application.Service
}

type recordingBookingRepo struct {
	inner ports.Repository[domain.BookingID, domain.BookingSnapshot]
	rec   *serviceskit.Steps
}

func (r recordingBookingRepo) Load(ctx context.Context, id domain.BookingID) (domain.BookingSnapshot, ports.Version, error) {
	r.rec.Record("bookings.Load")
	return r.inner.Load(ctx, id)
}

func (r recordingBookingRepo) Save(ctx context.Context, id domain.BookingID, s domain.BookingSnapshot, expected ports.Version) error {
	r.rec.Record("bookings.Save")
	return r.inner.Save(ctx, id, s, expected)
}

type recordingResourceRepo struct {
	inner ports.Repository[domain.ResourceCode, domain.ResourceSnapshot]
	rec   *serviceskit.Steps
}

func (r recordingResourceRepo) Load(ctx context.Context, code domain.ResourceCode) (domain.ResourceSnapshot, ports.Version, error) {
	r.rec.Record("resources.Load")
	return r.inner.Load(ctx, code)
}

func (r recordingResourceRepo) Save(ctx context.Context, code domain.ResourceCode, s domain.ResourceSnapshot, expected ports.Version) error {
	r.rec.Record("resources.Save")
	return r.inner.Save(ctx, code, s, expected)
}

type stubResourceReader struct{}

func (stubResourceReader) LoadByResource(_ context.Context, _ domain.ResourceID) ([]domain.BookingSnapshot, error) {
	return nil, nil
}

type option func(*setup)

type setup struct {
	faults      serviceskit.Faults
	registerErr error
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

func newHarness(t *testing.T, options ...option) *harness {
	t.Helper()

	fakes := serviceskit.NewFakes()
	h := &harness{fakes: fakes, store: fakes.Store, rec: fakes.Steps}
	cfg := &setup{}
	for _, apply := range options {
		apply(cfg)
	}
	fakes.FailRegister = cfg.registerErr

	bind := func(tx serviceskit.Tx) application.Resources {
		return application.Resources{
			Bookings:  recordingBookingRepo{inner: serviceskit.FaultyRepository(bookingTable.Repository(tx.Memory()), cfg.faults), rec: h.rec},
			Resources: recordingResourceRepo{inner: serviceskit.FaultyRepository(resourceTable.Repository(tx.Memory()), cfg.faults), rec: h.rec},
			Outbox:    serviceskit.FaultyOutbox(tx.Outbox(), cfg.faults),
			Commands:  tx.CommandInbox(application.CommandConsumer),
		}
	}

	h.service = application.Service{
		UoW:            serviceskit.UnitOfWork(fakes, bind),
		Reader:         bookingTable.Reader(h.store),
		ResourceReader: stubResourceReader{},
		Clock:          fakes.Clock(memory.FixedClock{At: testOccurred}),
		IDs:            fakes.IDs(&memory.SequenceIDs{Prefix: "m-"}),
		Authorize:      serviceskit.Authorize(fakes, usecase.AllowAll[application.Operation]()),
		Idempotency:    usecase.IdempotencyPolicy{Wait: 1_000_000_000, Retention: 86_400_000_000_000, Digest: serviceskit.FoldDigest},
	}
	return h
}

func (h *harness) seedBooking(t *testing.T, snap domain.BookingSnapshot, version ports.Version) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, func(tx *memory.Tx) application.Resources {
		return application.Resources{Bookings: bookingTable.Repository(tx)}
	})
	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res application.Resources) error {
		for expected := range version {
			if err := res.Bookings.Save(ctx, snap.ID, snap, expected); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seedBooking: %v", err)
	}
}
