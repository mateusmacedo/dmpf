package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Table names one logical table of the Store and how its snapshots cross the
// boundary. A Reader and a Repository built from the same Table see the same
// rows; one Name binds one pair of ID and S for the life of the store.
type Table[ID comparable, S any] struct {
	Name string

	// Clone copies a snapshot on every crossing of the boundary, so a UPR that
	// mutates the state it loaded cannot reach the store before the commit. Nil
	// means the value copy is the whole clone: S holds no slice, map or pointer.
	Clone func(S) S
}

// Reader is the read-only view outside any transaction, for the query of
// UOW-11. Called from inside a callback it returns the committed state, which
// is the state a reader outside the transaction would see.
func (t Table[ID, S]) Reader(s *Store) ports.Reader[ID, S] {
	return storeReader[ID, S]{store: s, table: t}
}

func (t Table[ID, S]) Repository(tx *Tx) ports.Repository[ID, S] {
	return txRepository[ID, S]{tx: tx, table: t}
}

func (t Table[ID, S]) copy(s S) S {
	if t.Clone == nil {
		return s
	}
	return t.Clone(s)
}

func (t Table[ID, S]) erasedClone() func(any) any {
	if t.Clone == nil {
		return nil
	}
	return func(v any) any { return t.Clone(v.(S)) }
}

func (t Table[ID, S]) load(from map[string]*table, id ID) (S, ports.Version, error) {
	var zero S
	rows, ok := from[t.Name]
	if !ok {
		return zero, 0, ports.ErrNotFound
	}
	rec, ok := rows.rows[id]
	if !ok {
		return zero, 0, ports.ErrNotFound
	}
	return t.copy(rec.snapshot.(S)), rec.version, nil
}

type row struct {
	snapshot any
	version  ports.Version
}

// table is the storage behind one Table: the rows by ID and the clone they
// need when the whole table is copied at open and at commit.
type table struct {
	rows  map[any]row
	clone func(any) any
}

// Store is the single resource this realization transacts over. Two mutexes:
// txMu serializes transactions and is held for the whole callback, dataMu
// protects the state and is held per read or write. One mutex doing both jobs
// would deadlock a Table.Reader load made from inside a callback.
type Store struct {
	txMu   sync.Mutex
	dataMu sync.Mutex

	tables         map[string]*table
	inbox          map[inboxKey]inboxRow
	outbox         []ports.OutboxEntry
	withinCalls    int
	commits        int
	failNextCommit error
}

// New builds an empty store.
func New() *Store {
	return &Store{
		tables: map[string]*table{},
		inbox:  map[inboxKey]inboxRow{},
	}
}

// InboxRows counts the committed inbox rows, for inspection by tests.
func (s *Store) InboxRows() int {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	return len(s.inbox)
}

// InboxStatus reads the committed status of one consumer/id pair, for
// inspection by tests; ok is false when the pair was never committed.
func (s *Store) InboxStatus(consumer string, id ports.MessageID) (ports.Status, bool) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	row, ok := s.inbox[inboxKey{consumer: consumer, id: id}]
	return row.status, ok
}

// InboxLastError reads the committed Completion.LastError of one
// consumer/id pair, for inspection by tests.
func (s *Store) InboxLastError(consumer string, id ports.MessageID) (string, bool) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	row, ok := s.inbox[inboxKey{consumer: consumer, id: id}]
	return row.lastError, ok
}

// Entries copies the committed outbox, for inspection by tests.
func (s *Store) Entries() []ports.OutboxEntry {
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

type storeReader[ID comparable, S any] struct {
	store *Store
	table Table[ID, S]
}

func (r storeReader[ID, S]) Load(_ context.Context, id ID) (S, ports.Version, error) {
	r.store.dataMu.Lock()
	defer r.store.dataMu.Unlock()
	return r.table.load(r.store.tables, id)
}

func (t *table) cloneRows() *table {
	out := &table{rows: make(map[any]row, len(t.rows)), clone: t.clone}
	for id, rec := range t.rows {
		if t.clone != nil {
			rec.snapshot = t.clone(rec.snapshot)
		}
		out.rows[id] = rec
	}
	return out
}

func cloneTables(src map[string]*table) map[string]*table {
	out := make(map[string]*table, len(src))
	for name, rows := range src {
		out[name] = rows.cloneRows()
	}
	return out
}

func cloneInboxRows(src map[inboxKey]inboxRow) map[inboxKey]inboxRow {
	out := make(map[inboxKey]inboxRow, len(src))
	for key, row := range src {
		out[key] = row
	}
	return out
}
