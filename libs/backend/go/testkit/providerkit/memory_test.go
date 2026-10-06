package providerkit_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// The in-memory realization of application is the first candidate of the
// suites: it needs no infrastructure, and it is what serviceskit builds on.

func TestMemoryUnitOfWorkConforms(t *testing.T) {
	v := providerkit.UnitOfWork(providerkit.MemoryUnitOfWork)
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("memory can inject a commit failure; nothing should be skipped: %v", v.Skipped)
	}
}

type probe struct{ Marker int }

type probeResources struct {
	Probes ports.Repository[string, probe]
}

var probeTable = memory.Table[string, probe]{Name: "probes"}

func memoryRepository() providerkit.RepositorySubject[string, probe] {
	store := memory.New()
	return providerkit.RepositorySubject[string, probe]{
		Within: func(ctx context.Context, fn func(ctx context.Context, repo ports.Repository[string, probe]) error) error {
			uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) probeResources {
				return probeResources{Probes: probeTable.Repository(tx)}
			})
			return uow.Within(ctx, func(ctx context.Context, res probeResources) error { return fn(ctx, res.Probes) })
		},
		Reader:           probeTable.Reader(store),
		NewID:            func(n int) string { return "p-" + strconv.Itoa(n) },
		NewState:         func(marker int) probe { return probe{Marker: marker} },
		Marker:           func(p probe) int { return p.Marker },
		TenantUnresolved: memory.ErrTenantUnresolved,
	}
}

func TestMemoryRepositoryConforms(t *testing.T) {
	v := providerkit.Repository(memoryRepository)
	tb.Require(t, v)
	if want := []string{"lets exactly one of two concurrent writers through"}; !slices.Equal(v.Skipped, want) {
		t.Fatalf("skipped = %v, want %v: memory scopes by construction and serializes Within by design", v.Skipped, want)
	}
}

func TestAConcurrentClaimOverASerializingRealizationFailsInsteadOfHanging(t *testing.T) {
	defer providerkit.ShortenConcurrentBarrier(100 * time.Millisecond)()
	forced := func() providerkit.RepositorySubject[string, probe] {
		s := memoryRepository()
		s.Concurrent = true
		return s
	}

	v := providerkit.Repository(forced)

	if got := concurrentDiagnostics(v); len(got) != 1 || !strings.Contains(got[0], "Within serializes callers") {
		t.Fatalf("diagnostics = %v, want the clause to name the serialization", got)
	}
}

func TestAConcurrentWriterWhoseTransactionNeverOpensIsNamedNotBlamedOnSerialization(t *testing.T) {
	defer providerkit.ShortenConcurrentBarrier(100 * time.Millisecond)()
	errBegin := errors.New("begin refused")
	failingSecondWriter := func() providerkit.RepositorySubject[string, probe] {
		s := memoryRepository()
		s.Concurrent = true
		within := s.Within
		var calls atomic.Int32
		s.Within = func(ctx context.Context, fn func(ctx context.Context, repo ports.Repository[string, probe]) error) error {
			if calls.Add(1) == 3 {
				return errBegin
			}
			return within(ctx, fn)
		}
		return s
	}

	v := providerkit.Repository(failingSecondWriter)

	if got := concurrentDiagnostics(v); len(got) != 1 || !strings.Contains(got[0], errBegin.Error()) {
		t.Fatalf("diagnostics = %v, want the clause to name %q", got, errBegin)
	}
}

func concurrentDiagnostics(v providerkit.Verdict) []string {
	var diagnostics []string
	for _, d := range v.Diagnostics {
		if d.Clause == "lets exactly one of two concurrent writers through" {
			diagnostics = append(diagnostics, d.Detail)
		}
	}
	return diagnostics
}

// unscopedRepository is the negative vector of IDN-14: it keys rows by
// identifier alone, so one tenant reads and overwrites another's row. The suite
// must name IDN-13.
type unscopedRepository struct {
	rows map[string]row
}

type row struct {
	state   probe
	version ports.Version
}

func (r *unscopedRepository) Load(_ context.Context, id string) (probe, ports.Version, error) {
	rec, ok := r.rows[id]
	if !ok {
		return probe{}, 0, ports.ErrNotFound
	}
	return rec.state, rec.version, nil
}

func (r *unscopedRepository) Save(_ context.Context, id string, state probe, expected ports.Version) error {
	if r.rows[id].version != expected {
		return ports.ErrVersionConflict
	}
	r.rows[id] = row{state: state, version: expected + 1}
	return nil
}

func TestAnUnscopedRepositoryIsReprovedOnIDN13(t *testing.T) {
	v := providerkit.Repository(func() providerkit.RepositorySubject[string, probe] {
		unscoped := &unscopedRepository{rows: map[string]row{}}
		s := memoryRepository()
		s.Within = func(ctx context.Context, fn func(ctx context.Context, repo ports.Repository[string, probe]) error) error {
			return fn(ctx, unscoped)
		}
		s.Reader = unscoped
		return s
	})
	if v.OK() {
		t.Fatal("a repository that keys rows by identifier alone passed the scope clauses")
	}
	var named bool
	for _, d := range v.Diagnostics {
		if d.Rule == "IDN-13" {
			named = true
		}
	}
	if !named {
		t.Fatalf("IDN-13 not named: %v", v.Failures())
	}
}

// silentReader is the negative vector of IDN-12: it scopes correctly, so no
// row leaks, but answers every miss with a bare ErrNotFound and leaves the
// security record nothing to name. The suite must name IDN-12.
type silentReader struct{ inner ports.Reader[string, probe] }

func (r silentReader) Load(ctx context.Context, id string) (probe, ports.Version, error) {
	state, version, err := r.inner.Load(ctx, id)
	if errors.Is(err, ports.ErrNotFound) {
		return probe{}, 0, ports.ErrNotFound
	}
	return state, version, err
}

func TestAScopedButSilentReaderIsReprovedOnIDN12(t *testing.T) {
	v := providerkit.Repository(func() providerkit.RepositorySubject[string, probe] {
		s := memoryRepository()
		s.Reader = silentReader{inner: s.Reader}
		return s
	})
	if v.OK() {
		t.Fatal("a reader that cannot tell another tenant's row from an absent one passed the scope clauses")
	}
	var named bool
	for _, d := range v.Diagnostics {
		if d.Rule == "IDN-12" {
			named = true
		}
	}
	if !named {
		t.Fatalf("IDN-12 not named: %v", v.Failures())
	}
}

func TestMemoryInboxConforms(t *testing.T) {
	v := providerkit.Inbox(providerkit.MemoryInbox)
	tb.Require(t, v)
	if len(v.Skipped) != 1 {
		t.Fatalf("expected exactly the concurrency clause skipped, got %v", v.Skipped)
	}
}

// A unit of work that commits even when the callback fails: the suite must
// name UOW-06.
type lenientUoW struct {
	store *memory.Store
	inner ports.UnitOfWork[providerkit.OutboxResources]
}

func (u lenientUoW) Within(ctx context.Context, fn func(context.Context, providerkit.OutboxResources) error) error {
	var cbErr error
	err := u.inner.Within(ctx, func(ctx context.Context, res providerkit.OutboxResources) error {
		cbErr = fn(ctx, res)
		return nil
	})
	return errors.Join(err, cbErr)
}

func TestALenientUnitOfWorkIsReproved(t *testing.T) {
	v := providerkit.UnitOfWork(func() providerkit.UnitOfWorkSubject[providerkit.OutboxResources] {
		s := providerkit.MemoryUnitOfWork()
		store := memory.New()
		s.UoW = lenientUoW{store: store, inner: memory.NewUnitOfWork(store, func(tx *memory.Tx) providerkit.OutboxResources {
			return providerkit.OutboxResources{Outbox: tx.Outbox()}
		})}
		s.Kept = func() int { return len(store.Entries()) }
		s.ArmCommitFailure = store.FailNextCommit
		return s
	})
	if v.OK() {
		t.Fatal("a unit of work that commits a failing callback passed")
	}
	var named bool
	for _, d := range v.Diagnostics {
		if d.Rule == "UOW-06" {
			named = true
		}
	}
	if !named {
		t.Fatalf("UOW-06 not named: %v", v.Failures())
	}
}

// racyInbox is the negative vector of INB-06: it checks the key and then
// inserts without holding anything in between, so two concurrent first
// receptions both see the first branch. The suite must name INB-06.
type racyInbox struct {
	mu        sync.Mutex
	committed map[string]ports.Status
}

type racyPending struct {
	inbox *racyInbox
	key   string
	done  bool
}

func (p *racyPending) Complete(_ context.Context, c ports.Completion) error {
	p.inbox.mu.Lock()
	defer p.inbox.mu.Unlock()
	p.inbox.committed[p.key] = c.Status
	p.done = true
	return nil
}

func (p *racyPending) Completed() bool { return p.done }

type racyBound struct {
	inbox    *racyInbox
	consumer string
}

func (b racyBound) Register(_ context.Context, r ports.Receipt) (ports.Reception, error) {
	if r.Consumer != b.consumer {
		return ports.Reception{}, errors.New("racy: consumer mismatch")
	}
	key := b.consumer + "/" + string(r.MessageID)
	b.inbox.mu.Lock()
	status, present := b.inbox.committed[key]
	b.inbox.mu.Unlock()
	// The check is done and the lock released before the insert: this is the
	// window INB-06 forbids.
	if present {
		if status == ports.StatusProcessed {
			return ports.ProcessedReception(), nil
		}
		return ports.RejectedReception(), nil
	}
	return ports.FirstReception(&racyPending{inbox: b.inbox, key: key}), nil
}

func racyInboxSubject() providerkit.InboxSubject {
	in := &racyInbox{committed: map[string]ports.Status{}}
	return providerkit.InboxSubject{
		Within: func(ctx context.Context, consumer string, fn func(ctx context.Context, inbox ports.Inbox) error) error {
			return fn(ctx, racyBound{inbox: in, consumer: consumer})
		},
		ReadStatus: func(consumer string, id ports.MessageID) (ports.Status, bool) {
			in.mu.Lock()
			defer in.mu.Unlock()
			s, ok := in.committed[consumer+"/"+string(id)]
			return s, ok
		},
		Concurrent: true,
	}
}

func TestACheckThenInsertInboxIsReprovedOnINB06(t *testing.T) {
	v := providerkit.Inbox(racyInboxSubject)
	var named bool
	for _, d := range v.Diagnostics {
		if d.Rule == "INB-06" && strings.Contains(d.Detail, "saw the first branch") {
			named = true
		}
	}
	if !named {
		t.Fatalf("a check-then-insert inbox was not reproved on INB-06: %v", v.Failures())
	}
}
