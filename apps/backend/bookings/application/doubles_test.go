package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
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

type recorder struct {
	mu       sync.Mutex
	observed []string
}

func (r *recorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observed = append(r.observed, name)
}

type harness struct {
	store   *memory.Store
	rec     *recorder
	service application.Service
}

type recordingBookingRepo struct {
	inner ports.Repository[domain.BookingID, domain.BookingSnapshot]
	rec   *recorder
}

func (r recordingBookingRepo) Load(ctx context.Context, id domain.BookingID) (domain.BookingSnapshot, ports.Version, error) {
	r.rec.record("bookings.Load")
	return r.inner.Load(ctx, id)
}

func (r recordingBookingRepo) Save(ctx context.Context, id domain.BookingID, s domain.BookingSnapshot, expected ports.Version) error {
	r.rec.record("bookings.Save")
	return r.inner.Save(ctx, id, s, expected)
}

type recordingResourceRepo struct {
	inner ports.Repository[domain.ResourceCode, domain.ResourceSnapshot]
	rec   *recorder
}

func (r recordingResourceRepo) Load(ctx context.Context, code domain.ResourceCode) (domain.ResourceSnapshot, ports.Version, error) {
	r.rec.record("resources.Load")
	return r.inner.Load(ctx, code)
}

func (r recordingResourceRepo) Save(ctx context.Context, code domain.ResourceCode, s domain.ResourceSnapshot, expected ports.Version) error {
	r.rec.record("resources.Save")
	return r.inner.Save(ctx, code, s, expected)
}

type recordingOutbox struct {
	inner ports.Outbox
	rec   *recorder
}

func (o recordingOutbox) Enqueue(ctx context.Context, entry ports.OutboxEntry) error {
	o.rec.record("outbox.Enqueue")
	return o.inner.Enqueue(ctx, entry)
}

var errFirstReception = errors.New("first reception")

type recordingCommands struct {
	inner       ports.Inbox
	rec         *recorder
	registerErr error
}

func (c recordingCommands) Register(ctx context.Context, r ports.Receipt) (ports.Reception, error) {
	c.rec.record("commands.Register")
	if c.registerErr != nil {
		return ports.Reception{}, c.registerErr
	}
	reception, err := c.inner.Register(ctx, r)
	if err != nil {
		return reception, err
	}
	var pending ports.Pending
	capture := func(p ports.Pending) error { pending = p; return errFirstReception }
	keep := func() error { return nil }
	if err := reception.Match(capture, keep, keep, keep); !errors.Is(err, errFirstReception) {
		return reception, err
	}
	return ports.FirstReception(recordingPending{inner: pending, rec: c.rec}), nil
}

type recordingPending struct {
	inner ports.Pending
	rec   *recorder
}

func (p recordingPending) Complete(ctx context.Context, c ports.Completion) error {
	p.rec.record("commands.Complete")
	return p.inner.Complete(ctx, c)
}

func (p recordingPending) Completed() bool { return p.inner.Completed() }

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

type stubResourceReader struct{}

func (stubResourceReader) LoadByResource(_ context.Context, _ domain.ResourceID) ([]domain.BookingSnapshot, error) {
	return nil, nil
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

func foldDigest(canonical []byte) ports.Fingerprint {
	var fingerprint ports.Fingerprint
	for i, b := range canonical {
		fingerprint[i%len(fingerprint)] = fingerprint[i%len(fingerprint)]*31 + b
	}
	return fingerprint
}

type option func(*setup)

type setup struct {
	registerErr error
}

func withRegisterError(err error) option {
	return func(s *setup) { s.registerErr = err }
}

func newHarness(t *testing.T, options ...option) *harness {
	t.Helper()

	h := &harness{store: memory.New(), rec: &recorder{}}
	cfg := &setup{}
	for _, apply := range options {
		apply(cfg)
	}

	bind := func(tx *memory.Tx) application.Resources {
		return application.Resources{
			Bookings:  recordingBookingRepo{inner: bookingTable.Repository(tx), rec: h.rec},
			Resources: recordingResourceRepo{inner: resourceTable.Repository(tx), rec: h.rec},
			Outbox:    recordingOutbox{inner: tx.Outbox(), rec: h.rec},
			Commands: recordingCommands{
				inner: tx.CommandInbox(application.CommandConsumer), rec: h.rec, registerErr: cfg.registerErr,
			},
		}
	}

	h.service = application.Service{
		UoW:            recordingUnitOfWork[application.Resources]{inner: memory.NewUnitOfWork(h.store, bind), rec: h.rec},
		Reader:         bookingTable.Reader(h.store),
		ResourceReader: stubResourceReader{},
		Clock:          recordingClock{inner: memory.FixedClock{At: testOccurred}, rec: h.rec},
		IDs:            recordingIDs{inner: &memory.SequenceIDs{Prefix: "m-"}, rec: h.rec},
		Authorize:      recordingAuthorize(h.rec, usecase.AllowAll[application.Operation]()),
		Idempotency:    usecase.IdempotencyPolicy{Wait: 1_000_000_000, Retention: 86_400_000_000_000, Digest: foldDigest},
	}
	return h
}

func recordingAuthorize(rec *recorder, inner usecase.Authorize[application.Operation]) usecase.Authorize[application.Operation] {
	return func(ctx context.Context, cmd application.Operation) error {
		rec.record("authorize")
		return inner(ctx, cmd)
	}
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
