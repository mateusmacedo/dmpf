package bookingsapplication_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"

	bookingsapplication "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/application"
	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

const (
	testBookingID  = bookingsdomain.BookingID("B-100")
	testResourceID = bookingsdomain.ResourceID("R-200")
	testResCode    = bookingsdomain.ResourceCode("room-101")
)

var testOccurred = dmpfports.Instant(1755432000)

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
	snapshot bookingsdomain.BookingSnapshot
	version  dmpfports.Version
}

type resourceRecord struct {
	snapshot bookingsdomain.ResourceSnapshot
	version  dmpfports.Version
}

type memStore struct {
	mu        sync.Mutex
	bookings  map[bookingsdomain.BookingID]bookingRecord
	resources map[bookingsdomain.ResourceCode]resourceRecord
	outbox    []dmpfports.OutboxEntry
}

func newStore() *memStore {
	return &memStore{
		bookings:  map[bookingsdomain.BookingID]bookingRecord{},
		resources: map[bookingsdomain.ResourceCode]resourceRecord{},
	}
}

type memTx struct {
	store     *memStore
	bookings  map[bookingsdomain.BookingID]bookingRecord
	resources map[bookingsdomain.ResourceCode]resourceRecord
	outbox    []dmpfports.OutboxEntry
}

func (s *memStore) newTx() *memTx {
	s.mu.Lock()
	defer s.mu.Unlock()
	bk := make(map[bookingsdomain.BookingID]bookingRecord, len(s.bookings))
	for k, v := range s.bookings {
		bk[k] = v
	}
	rs := make(map[bookingsdomain.ResourceCode]resourceRecord, len(s.resources))
	for k, v := range s.resources {
		rs[k] = v
	}
	return &memTx{store: s, bookings: bk, resources: rs}
}

func (t *memTx) commit() {
	t.store.mu.Lock()
	defer t.store.mu.Unlock()
	bk := make(map[bookingsdomain.BookingID]bookingRecord, len(t.bookings))
	for k, v := range t.bookings {
		bk[k] = v
	}
	t.store.bookings = bk
	rs := make(map[bookingsdomain.ResourceCode]resourceRecord, len(t.resources))
	for k, v := range t.resources {
		rs[k] = v
	}
	t.store.resources = rs
	t.store.outbox = append(t.store.outbox, t.outbox...)
}

type txBookings struct{ tx *memTx }

func (r txBookings) Load(_ context.Context, id bookingsdomain.BookingID) (bookingsdomain.BookingSnapshot, dmpfports.Version, error) {
	rec, ok := r.tx.bookings[id]
	if !ok {
		return bookingsdomain.BookingSnapshot{}, 0, dmpfports.ErrNotFound
	}
	return rec.snapshot, rec.version, nil
}

func (r txBookings) Save(_ context.Context, id bookingsdomain.BookingID, s bookingsdomain.BookingSnapshot, expected dmpfports.Version) error {
	current := dmpfports.Version(0)
	if rec, ok := r.tx.bookings[id]; ok {
		current = rec.version
	}
	if current != expected {
		return dmpfports.ErrVersionConflict
	}
	r.tx.bookings[id] = bookingRecord{snapshot: s, version: expected + 1}
	return nil
}

type txResources struct{ tx *memTx }

func (r txResources) Load(_ context.Context, code bookingsdomain.ResourceCode) (bookingsdomain.ResourceSnapshot, dmpfports.Version, error) {
	rec, ok := r.tx.resources[code]
	if !ok {
		return bookingsdomain.ResourceSnapshot{}, 0, dmpfports.ErrNotFound
	}
	return rec.snapshot, rec.version, nil
}

func (r txResources) Save(_ context.Context, code bookingsdomain.ResourceCode, s bookingsdomain.ResourceSnapshot, expected dmpfports.Version) error {
	current := dmpfports.Version(0)
	if rec, ok := r.tx.resources[code]; ok {
		current = rec.version
	}
	if current != expected {
		return dmpfports.ErrVersionConflict
	}
	r.tx.resources[code] = resourceRecord{snapshot: s, version: expected + 1}
	return nil
}

type txOutbox struct{ tx *memTx }

func (o txOutbox) Enqueue(_ context.Context, entry dmpfports.OutboxEntry) error {
	o.tx.outbox = append(o.tx.outbox, entry)
	return nil
}

type memUoW struct {
	store *memStore
	rec   *recorder
}

func (u memUoW) Within(ctx context.Context, fn func(context.Context, bookingsapplication.Resources) error) error {
	if u.rec != nil {
		u.rec.record("within")
	}
	tx := u.store.newTx()
	res := bookingsapplication.Resources{
		Bookings:  recordingBookingRepo{inner: txBookings{tx: tx}, rec: u.rec},
		Resources: recordingResourceRepo{inner: txResources{tx: tx}, rec: u.rec},
		Outbox:    recordingOutbox{inner: txOutbox{tx: tx}, rec: u.rec},
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

func (r storeReader) Load(_ context.Context, id bookingsdomain.BookingID) (bookingsdomain.BookingSnapshot, dmpfports.Version, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	rec, ok := r.store.bookings[id]
	if !ok {
		return bookingsdomain.BookingSnapshot{}, 0, dmpfports.ErrNotFound
	}
	return rec.snapshot, rec.version, nil
}

type stubResourceReader struct{}

func (stubResourceReader) LoadByResource(_ context.Context, _ bookingsdomain.ResourceID) ([]bookingsdomain.BookingSnapshot, error) {
	return nil, nil
}

type recordingBookingRepo struct {
	inner txBookings
	rec   *recorder
}

func (r recordingBookingRepo) Load(ctx context.Context, id bookingsdomain.BookingID) (bookingsdomain.BookingSnapshot, dmpfports.Version, error) {
	if r.rec != nil {
		r.rec.record("bookings.Load")
	}
	return r.inner.Load(ctx, id)
}

func (r recordingBookingRepo) Save(ctx context.Context, id bookingsdomain.BookingID, s bookingsdomain.BookingSnapshot, expected dmpfports.Version) error {
	if r.rec != nil {
		r.rec.record("bookings.Save")
	}
	return r.inner.Save(ctx, id, s, expected)
}

type recordingResourceRepo struct {
	inner txResources
	rec   *recorder
}

func (r recordingResourceRepo) Load(ctx context.Context, code bookingsdomain.ResourceCode) (bookingsdomain.ResourceSnapshot, dmpfports.Version, error) {
	if r.rec != nil {
		r.rec.record("resources.Load")
	}
	return r.inner.Load(ctx, code)
}

func (r recordingResourceRepo) Save(ctx context.Context, code bookingsdomain.ResourceCode, s bookingsdomain.ResourceSnapshot, expected dmpfports.Version) error {
	if r.rec != nil {
		r.rec.record("resources.Save")
	}
	return r.inner.Save(ctx, code, s, expected)
}

type recordingOutbox struct {
	inner txOutbox
	rec   *recorder
}

func (o recordingOutbox) Enqueue(ctx context.Context, entry dmpfports.OutboxEntry) error {
	if o.rec != nil {
		o.rec.record("outbox.Enqueue")
	}
	return o.inner.Enqueue(ctx, entry)
}

type recordingClock struct {
	at  dmpfports.Instant
	rec *recorder
}

func (c recordingClock) Now() dmpfports.Instant {
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

func (g *recordingIDs) NewMessageID() dmpfports.MessageID {
	if g.rec != nil {
		g.rec.record("ids.NewMessageID")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return dmpfports.MessageID(fmt.Sprintf("%s%06d", g.prefix, g.counter))
}

type harness struct {
	store   *memStore
	rec     *recorder
	service bookingsapplication.Service
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store := newStore()
	rec := &recorder{}

	h := &harness{store: store, rec: rec}
	h.service = bookingsapplication.Service{
		UoW:            memUoW{store: store, rec: rec},
		Reader:         storeReader{store: store},
		ResourceReader: stubResourceReader{},
		Clock:          recordingClock{at: testOccurred, rec: rec},
		IDs:            &recordingIDs{prefix: "m-", rec: rec},
		Authorize:      recordingAuthorize(rec, dmpfapplication.AllowAll[bookingsapplication.Command]()),
	}
	return h
}

func recordingAuthorize(rec *recorder, inner dmpfapplication.AuthorizeFunc[bookingsapplication.Command]) dmpfapplication.AuthorizeFunc[bookingsapplication.Command] {
	return func(ctx context.Context, cmd bookingsapplication.Command) error {
		rec.record("authorize")
		return inner(ctx, cmd)
	}
}

func (h *harness) seedBooking(t *testing.T, snap bookingsdomain.BookingSnapshot, version dmpfports.Version) {
	t.Helper()
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	h.store.bookings[snap.ID] = bookingRecord{snapshot: snap, version: version}
}
