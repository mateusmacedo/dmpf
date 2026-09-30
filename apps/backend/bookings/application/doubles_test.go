package application_test

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	testBookingID  = domain.BookingID("B-100")
	testResourceID = domain.ResourceID("R-200")
	testResCode    = domain.ResourceCode("room-101")
)

var testOccurred = ports.Instant(1755432000)

type recorder struct {
	mu       sync.Mutex
	observed []string
}

func (r *recorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observed = append(r.observed, name)
}

type bookingRecord struct {
	snapshot domain.BookingSnapshot
	version  ports.Version
}

type resourceRecord struct {
	snapshot domain.ResourceSnapshot
	version  ports.Version
}

type commandRecord struct {
	hash    string
	status  ports.Status
	outcome []byte
}

type memStore struct {
	mu        sync.Mutex
	bookings  map[domain.BookingID]bookingRecord
	resources map[domain.ResourceCode]resourceRecord
	commands  map[ports.MessageID]commandRecord
	outbox    []ports.OutboxEntry
}

func newStore() *memStore {
	return &memStore{
		bookings:  map[domain.BookingID]bookingRecord{},
		resources: map[domain.ResourceCode]resourceRecord{},
		commands:  map[ports.MessageID]commandRecord{},
	}
}

type memTx struct {
	store     *memStore
	bookings  map[domain.BookingID]bookingRecord
	resources map[domain.ResourceCode]resourceRecord
	commands  map[ports.MessageID]commandRecord
	outbox    []ports.OutboxEntry
}

func (s *memStore) newTx() *memTx {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &memTx{store: s, bookings: maps.Clone(s.bookings), resources: maps.Clone(s.resources), commands: maps.Clone(s.commands)}
}

func (t *memTx) commit() {
	t.store.mu.Lock()
	defer t.store.mu.Unlock()
	t.store.bookings = maps.Clone(t.bookings)
	t.store.resources = maps.Clone(t.resources)
	t.store.commands = maps.Clone(t.commands)
	t.store.outbox = append(t.store.outbox, t.outbox...)
}

type txCommands struct {
	tx          *memTx
	registerErr error
	rec         *recorder
}

func (c txCommands) Register(_ context.Context, r ports.Receipt) (ports.Reception, error) {
	if c.rec != nil {
		c.rec.record("commands.Register")
	}
	if c.registerErr != nil {
		return ports.Reception{}, c.registerErr
	}
	existing, ok := c.tx.commands[r.MessageID]
	switch {
	case !ok:
		return ports.FirstReception(&commandPending{tx: c.tx, id: r.MessageID, hash: r.PayloadHash, rec: c.rec}), nil
	case existing.hash != r.PayloadHash:
		return ports.CollisionReception(), nil
	case existing.status == ports.StatusRejected:
		return ports.RejectedReception().WithStored(existing.outcome), nil
	default:
		return ports.ProcessedReception().WithStored(existing.outcome), nil
	}
}

type commandPending struct {
	tx        *memTx
	id        ports.MessageID
	hash      string
	rec       *recorder
	completed bool
}

func (p *commandPending) Complete(_ context.Context, c ports.Completion) error {
	if p.rec != nil {
		p.rec.record("commands.Complete")
	}
	p.tx.commands[p.id] = commandRecord{hash: p.hash, status: c.Status, outcome: c.Outcome}
	p.completed = true
	return nil
}

func (p *commandPending) Completed() bool { return p.completed }

type txBookings struct{ tx *memTx }

func (r txBookings) Load(_ context.Context, id domain.BookingID) (domain.BookingSnapshot, ports.Version, error) {
	rec, ok := r.tx.bookings[id]
	if !ok {
		return domain.BookingSnapshot{}, 0, ports.ErrNotFound
	}
	return rec.snapshot, rec.version, nil
}

func (r txBookings) Save(_ context.Context, id domain.BookingID, s domain.BookingSnapshot, expected ports.Version) error {
	current := ports.Version(0)
	if rec, ok := r.tx.bookings[id]; ok {
		current = rec.version
	}
	if current != expected {
		return ports.ErrVersionConflict
	}
	r.tx.bookings[id] = bookingRecord{snapshot: s, version: expected + 1}
	return nil
}

type txResources struct{ tx *memTx }

func (r txResources) Load(_ context.Context, code domain.ResourceCode) (domain.ResourceSnapshot, ports.Version, error) {
	rec, ok := r.tx.resources[code]
	if !ok {
		return domain.ResourceSnapshot{}, 0, ports.ErrNotFound
	}
	return rec.snapshot, rec.version, nil
}

func (r txResources) Save(_ context.Context, code domain.ResourceCode, s domain.ResourceSnapshot, expected ports.Version) error {
	current := ports.Version(0)
	if rec, ok := r.tx.resources[code]; ok {
		current = rec.version
	}
	if current != expected {
		return ports.ErrVersionConflict
	}
	r.tx.resources[code] = resourceRecord{snapshot: s, version: expected + 1}
	return nil
}

type txOutbox struct{ tx *memTx }

func (o txOutbox) Enqueue(_ context.Context, entry ports.OutboxEntry) error {
	o.tx.outbox = append(o.tx.outbox, entry)
	return nil
}

type memUoW struct {
	store       *memStore
	rec         *recorder
	registerErr error
}

func (u *memUoW) Within(ctx context.Context, fn func(context.Context, application.Resources) error) error {
	if u.rec != nil {
		u.rec.record("within")
	}
	tx := u.store.newTx()
	res := application.Resources{
		Bookings:  recordingBookingRepo{inner: txBookings{tx: tx}, rec: u.rec},
		Resources: recordingResourceRepo{inner: txResources{tx: tx}, rec: u.rec},
		Outbox:    recordingOutbox{inner: txOutbox{tx: tx}, rec: u.rec},
		Commands:  txCommands{tx: tx, registerErr: u.registerErr, rec: u.rec},
	}
	if err := fn(ctx, res); err != nil {
		return err
	}
	tx.commit()
	if u.rec != nil {
		u.rec.record("commit")
	}
	return nil
}

type storeReader struct{ store *memStore }

func (r storeReader) Load(_ context.Context, id domain.BookingID) (domain.BookingSnapshot, ports.Version, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	rec, ok := r.store.bookings[id]
	if !ok {
		return domain.BookingSnapshot{}, 0, ports.ErrNotFound
	}
	return rec.snapshot, rec.version, nil
}

type stubResourceReader struct{}

func (stubResourceReader) LoadByResource(_ context.Context, _ domain.ResourceID) ([]domain.BookingSnapshot, error) {
	return nil, nil
}

type recordingBookingRepo struct {
	inner txBookings
	rec   *recorder
}

func (r recordingBookingRepo) Load(ctx context.Context, id domain.BookingID) (domain.BookingSnapshot, ports.Version, error) {
	if r.rec != nil {
		r.rec.record("bookings.Load")
	}
	return r.inner.Load(ctx, id)
}

func (r recordingBookingRepo) Save(ctx context.Context, id domain.BookingID, s domain.BookingSnapshot, expected ports.Version) error {
	if r.rec != nil {
		r.rec.record("bookings.Save")
	}
	return r.inner.Save(ctx, id, s, expected)
}

type recordingResourceRepo struct {
	inner txResources
	rec   *recorder
}

func (r recordingResourceRepo) Load(ctx context.Context, code domain.ResourceCode) (domain.ResourceSnapshot, ports.Version, error) {
	if r.rec != nil {
		r.rec.record("resources.Load")
	}
	return r.inner.Load(ctx, code)
}

func (r recordingResourceRepo) Save(ctx context.Context, code domain.ResourceCode, s domain.ResourceSnapshot, expected ports.Version) error {
	if r.rec != nil {
		r.rec.record("resources.Save")
	}
	return r.inner.Save(ctx, code, s, expected)
}

type recordingOutbox struct {
	inner txOutbox
	rec   *recorder
}

func (o recordingOutbox) Enqueue(ctx context.Context, entry ports.OutboxEntry) error {
	if o.rec != nil {
		o.rec.record("outbox.Enqueue")
	}
	return o.inner.Enqueue(ctx, entry)
}

type recordingClock struct {
	at  ports.Instant
	rec *recorder
}

func (c recordingClock) Now() ports.Instant {
	if c.rec != nil {
		c.rec.record("clock.Now")
	}
	return c.at
}

type recordingIDs struct {
	mu      sync.Mutex
	counter int
	prefix  string
	rec     *recorder
}

func (g *recordingIDs) NewMessageID() ports.MessageID {
	if g.rec != nil {
		g.rec.record("ids.NewMessageID")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return ports.MessageID(fmt.Sprintf("%s%06d", g.prefix, g.counter))
}

type harness struct {
	store   *memStore
	rec     *recorder
	uow     *memUoW
	service application.Service
}

func foldDigest(canonical []byte) ports.Fingerprint {
	var fingerprint ports.Fingerprint
	for i, b := range canonical {
		fingerprint[i%len(fingerprint)] = fingerprint[i%len(fingerprint)]*31 + b
	}
	return fingerprint
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store := newStore()
	rec := &recorder{}

	h := &harness{store: store, rec: rec, uow: &memUoW{store: store, rec: rec}}
	h.service = application.Service{
		UoW:            h.uow,
		Reader:         storeReader{store: store},
		ResourceReader: stubResourceReader{},
		Clock:          recordingClock{at: testOccurred, rec: rec},
		IDs:            &recordingIDs{prefix: "m-", rec: rec},
		Authorize:      recordingAuthorize(rec, usecase.AllowAll[application.Operation]()),
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
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	h.store.bookings[snap.ID] = bookingRecord{snapshot: snap, version: version}
}
