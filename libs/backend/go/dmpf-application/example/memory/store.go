package memory

import (
	"context"
	"slices"
	"sync"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

type record struct {
	snapshot orders.Snapshot
	version  dmpfports.Version
}

// Store is the single resource this realization transacts over.
type Store struct {
	mu             sync.Mutex
	orders         map[orders.OrderID]record
	outbox         []dmpfports.OutboxEntry
	withinCalls    int
	failNextCommit error
}

// New builds an empty store.
func New() *Store {
	return &Store{orders: map[orders.OrderID]record{}}
}

// Reader is the read-only view outside any transaction, for the query of
// UOW-11. Calling it from inside Within on the same store deadlocks: the mutex
// that serializes transactions is held for the whole callback.
func (s *Store) Reader() dmpfports.Reader[orders.OrderID, orders.Snapshot] {
	return storeReader{store: s}
}

// Entries copies the committed outbox, for inspection by tests.
func (s *Store) Entries() []dmpfports.OutboxEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.outbox)
}

// FailNextCommit arms one commit to fail with err, which is how a test proves
// that neither business state nor outbox survives a failed commit (UOW-07).
// The injection is consumed by that commit; call it outside Within.
func (s *Store) FailNextCommit(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failNextCommit = err
}

// WithinCalls counts the transactions actually opened, which is how a test
// distinguishes "one transaction" from "none" (UOW-01, UOW-02, UOW-11).
func (s *Store) WithinCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.withinCalls
}

type storeReader struct{ store *Store }

func (r storeReader) Load(_ context.Context, id orders.OrderID) (orders.Snapshot, dmpfports.Version, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return load(r.store.orders, id)
}

func load(from map[orders.OrderID]record, id orders.OrderID) (orders.Snapshot, dmpfports.Version, error) {
	rec, ok := from[id]
	if !ok {
		return orders.Snapshot{}, 0, dmpfports.ErrNotFound
	}
	return cloneSnapshot(rec.snapshot), rec.version, nil
}

// cloneSnapshot copies Items on every crossing of the boundary, so a UPR that
// mutates the aggregate it loaded cannot reach the store before the commit.
func cloneSnapshot(s orders.Snapshot) orders.Snapshot {
	s.Items = slices.Clone(s.Items)
	return s
}

func cloneRecords(src map[orders.OrderID]record) map[orders.OrderID]record {
	out := make(map[orders.OrderID]record, len(src))
	for id, rec := range src {
		out[id] = record{snapshot: cloneSnapshot(rec.snapshot), version: rec.version}
	}
	return out
}
