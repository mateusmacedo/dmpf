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

// Store is the single resource this realization transacts over. Two mutexes:
// txMu serializes transactions and is held for the whole callback, dataMu
// protects the state and is held per read or write. One mutex doing both jobs
// would deadlock a Reader() call made from inside a callback.
type Store struct {
	txMu   sync.Mutex
	dataMu sync.Mutex

	orders         map[orders.OrderID]record
	outbox         []dmpfports.OutboxEntry
	withinCalls    int
	commits        int
	failNextCommit error
}

// New builds an empty store.
func New() *Store {
	return &Store{orders: map[orders.OrderID]record{}}
}

// Reader is the read-only view outside any transaction, for the query of
// UOW-11. Called from inside a callback it returns the committed state, which
// is the state a reader outside the transaction would see.
func (s *Store) Reader() dmpfports.Reader[orders.OrderID, orders.Snapshot] {
	return storeReader{store: s}
}

// Entries copies the committed outbox, for inspection by tests.
func (s *Store) Entries() []dmpfports.OutboxEntry {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	return slices.Clone(s.outbox)
}

// FailNextCommit arms one commit to fail with err, which is how a test proves
// that neither business state nor outbox survives a failed commit (UOW-07).
// The injection is consumed by that commit.
func (s *Store) FailNextCommit(err error) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	s.failNextCommit = err
}

// WithinCalls counts the transactions actually opened, which is how a test
// distinguishes "one transaction" from "none" (UOW-01, UOW-02, UOW-11).
func (s *Store) WithinCalls() int {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	return s.withinCalls
}

// Commits counts the commits that actually installed state. A test needs it
// apart from WithinCalls because under a refusal the commit still happens
// (UOW-05, UOW-06), and inferring that from a nil error would also accept a
// realization that rolled back and returned nil.
func (s *Store) Commits() int {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	return s.commits
}

type storeReader struct{ store *Store }

func (r storeReader) Load(_ context.Context, id orders.OrderID) (orders.Snapshot, dmpfports.Version, error) {
	r.store.dataMu.Lock()
	defer r.store.dataMu.Unlock()
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
