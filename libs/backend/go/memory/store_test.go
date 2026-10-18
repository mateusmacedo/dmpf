package memory_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

// counter is the smallest aggregate that exercises every clause: Steps is the
// slice the no-shared-backing-array guarantee is about.
type counterID string

type counter struct {
	ID    counterID
	Total int
	Steps []int
}

func cloneCounter(c counter) counter {
	c.Steps = slices.Clone(c.Steps)
	return c
}

func equal(a, b counter) bool {
	return a.ID == b.ID && a.Total == b.Total && slices.Equal(a.Steps, b.Steps)
}

var counters = memory.Table[counterID, counter]{Name: "counters", Clone: cloneCounter}

// resources is what the composition root binds to an open transaction. It
// lives in the test because provider → application is a forbidden cell: the
// realization cannot know the use case's resource type.
type resources struct {
	Counters ports.Repository[counterID, counter]
	Outbox   ports.Outbox
}

func bind(tx *memory.Tx) resources {
	return resources{Counters: counters.Repository(tx), Outbox: tx.Outbox()}
}

const id = counterID("C-100")

var errBoom = errors.New("memory_test: boom")

func state(step int) counter {
	return counter{ID: id, Total: step, Steps: []int{step}}
}

func entry(messageID ports.MessageID) ports.OutboxEntry {
	return ports.OutboxEntry{
		MessageID:        messageID,
		OccurredAt:       1_755_432_000_000_000_000,
		Intent:           ports.PublishIntent{Destination: "counters.events", PartitionKey: string(id)},
		AggregateType:    "counters.Counter",
		AggregateID:      string(id),
		AggregateVersion: 1,
	}
}

func seed(t *testing.T, store *memory.Store, snapshot counter, expected ports.Version) {
	t.Helper()
	uow := memory.NewUnitOfWork(store, bind)
	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res resources) error {
		return res.Counters.Save(ctx, snapshot.ID, snapshot, expected)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestCommitAppliesBothWritesToTheStore(t *testing.T) {
	store := memory.New()
	uow := memory.NewUnitOfWork(store, bind)

	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res resources) error {
		if err := res.Counters.Save(ctx, id, state(1), 0); err != nil {
			return err
		}
		return res.Outbox.Enqueue(ctx, entry("m-000001"))
	})
	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	snapshot, version, err := counters.Reader(store).Load(withExecution(t, context.Background()), id)
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
	if !equal(snapshot, state(1)) {
		t.Fatalf("snapshot = %+v, want %+v", snapshot, state(1))
	}
	if got := store.Entries(); len(got) != 1 || got[0].MessageID != "m-000001" {
		t.Fatalf("Entries() = %+v, want exactly one entry m-000001", got)
	}
}

func TestAFailingCallbackKeepsNothing(t *testing.T) {
	store := memory.New()
	uow := memory.NewUnitOfWork(store, bind)

	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res resources) error {
		if err := res.Counters.Save(ctx, id, state(1), 0); err != nil {
			return err
		}
		if err := res.Outbox.Enqueue(ctx, entry("m-000001")); err != nil {
			return err
		}
		return errBoom
	})

	if !errors.Is(err, errBoom) {
		t.Fatalf("Within() = %v, want errBoom", err)
	}
	requireEmpty(t, store)
}

func TestFailNextCommitDiscardsTheTransactionAndReturnsTheError(t *testing.T) {
	store := memory.New()
	store.FailNextCommit(errBoom)
	uow := memory.NewUnitOfWork(store, bind)

	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res resources) error {
		if err := res.Counters.Save(ctx, id, state(1), 0); err != nil {
			return err
		}
		return res.Outbox.Enqueue(ctx, entry("m-000001"))
	})

	if !errors.Is(err, errBoom) {
		t.Fatalf("Within() = %v, want the injected errBoom", err)
	}
	requireEmpty(t, store)
}

func TestFailNextCommitArmsOnlyTheNextCommit(t *testing.T) {
	store := memory.New()
	store.FailNextCommit(errBoom)
	uow := memory.NewUnitOfWork(store, bind)
	noop := func(context.Context, resources) error { return nil }

	if err := uow.Within(withExecution(t, context.Background()), noop); !errors.Is(err, errBoom) {
		t.Fatalf("first Within() = %v, want errBoom", err)
	}
	if err := uow.Within(withExecution(t, context.Background()), noop); err != nil {
		t.Fatalf("second Within() = %v, want nil — the injection is consumed once", err)
	}
}

func TestAPanickingCallbackPropagatesAndKeepsNothing(t *testing.T) {
	store := memory.New()
	uow := memory.NewUnitOfWork(store, bind)

	func() {
		defer func() {
			if recovered := recover(); recovered != "memory_test: boom" {
				t.Fatalf("recovered %v, want \"memory_test: boom\"", recovered)
			}
		}()

		_ = uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res resources) error {
			if err := res.Counters.Save(ctx, id, state(1), 0); err != nil {
				return err
			}
			panic("memory_test: boom")
		})
	}()

	requireEmpty(t, store)
}

func TestACancelledContextNeverOpensATransaction(t *testing.T) {
	store := memory.New()
	uow := memory.NewUnitOfWork(store, bind)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	invoked := false

	err := uow.Within(ctx, func(context.Context, resources) error {
		invoked = true
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Within() = %v, want context.Canceled", err)
	}
	if invoked {
		t.Fatal("the callback ran on an already cancelled context")
	}
	if got := store.WithinCalls(); got != 0 {
		t.Fatalf("WithinCalls() = %d, want 0", got)
	}
}

func TestReaderSharesNoSliceWithTheStore(t *testing.T) {
	store := memory.New()
	seed(t, store, state(1), 0)

	loaded, _, err := counters.Reader(store).Load(withExecution(t, context.Background()), id)
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	loaded.Steps[0] = 999

	again, _, _ := counters.Reader(store).Load(withExecution(t, context.Background()), id)
	if again.Steps[0] != 1 {
		t.Fatalf("mutating a loaded snapshot reached the store: Steps[0] = %d, want 1", again.Steps[0])
	}
}

func TestSaveSharesNoSliceWithTheCaller(t *testing.T) {
	store := memory.New()
	snapshot := state(1)
	seed(t, store, snapshot, 0)

	snapshot.Steps[0] = 999

	loaded, _, _ := counters.Reader(store).Load(withExecution(t, context.Background()), id)
	if loaded.Steps[0] != 1 {
		t.Fatalf("mutating the caller's slice reached the store: Steps[0] = %d, want 1", loaded.Steps[0])
	}
}

// TestReaderInsideWithinReadsTheCommittedState pins the reason Store keeps two
// mutexes: with a single one serializing transactions and guarding the state,
// this read would deadlock instead of returning.
func TestReaderInsideWithinReadsTheCommittedState(t *testing.T) {
	store := memory.New()
	seed(t, store, state(1), 0)
	uow := memory.NewUnitOfWork(store, bind)

	var seenStep int
	var seenVersion ports.Version

	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res resources) error {
		if err := res.Counters.Save(ctx, id, state(7), 1); err != nil {
			return err
		}
		snapshot, version, err := counters.Reader(store).Load(ctx, id)
		if err != nil {
			return err
		}
		seenStep = snapshot.Steps[0]
		seenVersion = version
		return nil
	})
	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	if seenStep != 1 || seenVersion != 1 {
		t.Fatalf("Reader inside the callback saw step %d at v%d, want the committed 1 at v1",
			seenStep, seenVersion)
	}

	after, version, _ := counters.Reader(store).Load(withExecution(t, context.Background()), id)
	if after.Steps[0] != 7 || version != 2 {
		t.Fatalf("after the commit: step %d at v%d, want 7 at v2", after.Steps[0], version)
	}
}

// Without the clone in commit, a port kept past the callback would write
// straight into the committed state, outside any transaction and without dataMu.
func TestAPortThatEscapesTheCallbackCannotReachTheStore(t *testing.T) {
	store := memory.New()
	uow := memory.NewUnitOfWork(store, bind)

	var escaped resources
	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res resources) error {
		escaped = res
		return res.Counters.Save(ctx, id, state(1), 0)
	})
	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	if err := escaped.Counters.Save(withExecution(t, context.Background()), id, state(9), 1); err != nil {
		t.Fatalf("Save through the escaped port = %v, want nil (it writes the discarded copy)", err)
	}
	if err := escaped.Outbox.Enqueue(context.Background(), entry("m-999999")); err != nil {
		t.Fatalf("Enqueue through the escaped port = %v, want nil", err)
	}

	snapshot, version, _ := counters.Reader(store).Load(withExecution(t, context.Background()), id)
	if snapshot.Steps[0] != 1 || version != 1 {
		t.Fatalf("the escaped port reached the store: step %d at v%d, want 1 at v1",
			snapshot.Steps[0], version)
	}
	if got := store.Entries(); len(got) != 0 {
		t.Fatalf("the escaped port reached the outbox: %+v, want empty", got)
	}
}

// The first ctx.Err() check happens before waiting for txMu, so a caller whose
// context dies while queued has to be refused again when its turn comes.
func TestAContextCancelledWhileWaitingNeverOpensATransaction(t *testing.T) {
	store := memory.New()
	uow := memory.NewUnitOfWork(store, bind)
	ctx := &cancelledAfterFirstCheck{Context: context.Background()}

	holding := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = uow.Within(withExecution(t, context.Background()), func(context.Context, resources) error {
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding

	queued := make(chan error, 1)
	go func() {
		queued <- uow.Within(ctx, func(context.Context, resources) error {
			t.Error("the callback ran with a context cancelled while queued")
			return nil
		})
	}()

	close(release)

	if err := <-queued; !errors.Is(err, context.Canceled) {
		t.Fatalf("the queued Within() = %v, want context.Canceled", err)
	}
	if got := ctx.checks.Load(); got != 2 {
		t.Fatalf("ctx.Err() checked %d times, want 2 — the refusal must come after the wait", got)
	}
	if got := store.Commits(); got != 1 {
		t.Fatalf("Commits() = %d, want 1 — only the first transaction committed", got)
	}
}

type cancelledAfterFirstCheck struct {
	context.Context
	checks atomic.Int32
}

func (c *cancelledAfterFirstCheck) Err() error {
	if c.checks.Add(1) == 1 {
		return nil
	}
	return context.Canceled
}

func TestSaveRejectsADivergentExpectedVersion(t *testing.T) {
	store := memory.New()
	seed(t, store, state(1), 0)
	uow := memory.NewUnitOfWork(store, bind)

	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res resources) error {
		return res.Counters.Save(ctx, id, state(2), 0)
	})

	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Within() = %v, want ErrVersionConflict", err)
	}
	loaded, version, _ := counters.Reader(store).Load(withExecution(t, context.Background()), id)
	if version != 1 || loaded.Steps[0] != 1 {
		t.Fatalf("the store changed under a conflict: version = %d, step = %d", version, loaded.Steps[0])
	}
}

func TestLoadReportsErrNotFoundForAnAbsentAggregate(t *testing.T) {
	store := memory.New()

	_, _, err := counters.Reader(store).Load(withExecution(t, context.Background()), "C-404")

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound", err)
	}
}

// Two Tables with different names are two tables: a row saved in one is absent
// from the other, even under the same ID type.
func TestTablesWithDifferentNamesDoNotShareRows(t *testing.T) {
	store := memory.New()
	seed(t, store, state(1), 0)
	other := memory.Table[counterID, counter]{Name: "archive", Clone: cloneCounter}

	_, _, err := other.Reader(store).Load(withExecution(t, context.Background()), id)

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() through another table = %v, want ErrNotFound", err)
	}
}

// A Table without Clone stores the value as given, which is the whole clone
// when S carries no reference type.
func TestATableWithoutCloneRoundTripsTheValue(t *testing.T) {
	type flag struct{ On bool }
	flags := memory.Table[string, flag]{Name: "flags"}
	store := memory.New()
	uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) ports.Repository[string, flag] { return flags.Repository(tx) })

	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, repo ports.Repository[string, flag]) error {
		return repo.Save(ctx, "f-1", flag{On: true}, 0)
	})
	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	loaded, version, err := flags.Reader(store).Load(withExecution(t, context.Background()), "f-1")
	if err != nil || version != 1 || !loaded.On {
		t.Fatalf("Load() = (%+v, %d, %v), want (On, 1, nil)", loaded, version, err)
	}
}

func TestConcurrentWithinIsSerialized(t *testing.T) {
	const goroutines = 8
	store := memory.New()
	uow := memory.NewUnitOfWork(store, bind)
	var wg sync.WaitGroup

	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res resources) error {
				return res.Outbox.Enqueue(ctx, entry(ports.MessageID("m-"+string(rune('a'+i)))))
			})
		}(i)
	}
	wg.Wait()

	if got := store.WithinCalls(); got != goroutines {
		t.Fatalf("WithinCalls() = %d, want %d", got, goroutines)
	}
	if got := len(store.Entries()); got != goroutines {
		t.Fatalf("Entries() has %d entries, want %d", got, goroutines)
	}
}

func requireEmpty(t *testing.T, store *memory.Store) {
	t.Helper()
	if _, _, err := counters.Reader(store).Load(withExecution(t, context.Background()), id); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound — the store must be untouched", err)
	}
	if got := store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
}

type account struct {
	Email       string
	Org, Handle string
	Nick        string
	NickNull    bool
	Marker      int
}

var (
	emailTaken  = domain.Reject("accounts/email-taken", "the email is taken")
	handleTaken = domain.Reject("accounts/handle-taken", "the handle is taken in the org")
	nickTaken   = domain.Reject("accounts/nick-taken", "the nick is taken")
)

const (
	emailKey  = "accounts_tenant_id_email_key"
	handleKey = "accounts_tenant_id_org_handle_key"
	nickKey   = "accounts_tenant_id_nick_key"
)

var accounts = memory.Table[string, account]{
	Name: "accounts",
	Unique: []memory.Unique[account]{
		{Name: emailKey, Key: func(a account) ([]string, bool) { return []string{a.Email}, true }, Rejection: emailTaken},
		{Name: handleKey, Key: func(a account) ([]string, bool) { return []string{a.Org, a.Handle}, true }, Rejection: handleTaken},
		{Name: nickKey, Key: func(a account) ([]string, bool) { return []string{a.Nick}, !a.NickNull }, Rejection: nickTaken},
	},
}

type accountResources struct {
	Accounts ports.Repository[string, account]
	Outbox   ports.Outbox
	Inbox    ports.Inbox
}

func newAccount(n int) account {
	s := strconv.Itoa(n)
	return account{Email: "e-" + s, Org: "o", Handle: "h-" + s, Nick: "n-" + s, Marker: n}
}

func withinAccounts(t *testing.T, store *memory.Store, ctx context.Context, fn func(ctx context.Context, res accountResources) error) error {
	t.Helper()
	uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) accountResources {
		return accountResources{Accounts: accounts.Repository(tx), Outbox: tx.Outbox(), Inbox: tx.Inbox("accounts-consumer")}
	})
	return uow.Within(ctx, fn)
}

func seedAccount(t *testing.T, store *memory.Store, ctx context.Context, id string, a account) {
	t.Helper()
	if err := withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
		return res.Accounts.Save(ctx, id, a, 0)
	}); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func requireViolation(t *testing.T, err error, constraint string, rejection *domain.Rejection) {
	t.Helper()
	if !errors.Is(err, ports.ErrUniqueViolation) {
		t.Fatalf("Save() = %v, want ErrUniqueViolation", err)
	}
	violation, ok := errors.AsType[ports.UniqueViolation](err)
	if !ok {
		t.Fatalf("Save() = %v, want a UniqueViolation", err)
	}
	if violation.Table != "accounts" || violation.Constraint != constraint || violation.Rejection != rejection {
		t.Fatalf("violation = %+v, want accounts/%s carrying %v", violation, constraint, rejection)
	}
}

func TestEachDeclaredConstraintRefusesASecondRowWithItsOwnRejection(t *testing.T) {
	for _, tc := range []struct {
		constraint string
		rejection  *domain.Rejection
		collide    func(taken, a account) account
	}{
		{emailKey, emailTaken, func(taken, a account) account { a.Email = taken.Email; return a }},
		{handleKey, handleTaken, func(taken, a account) account { a.Org, a.Handle = taken.Org, taken.Handle; return a }},
		{nickKey, nickTaken, func(taken, a account) account { a.Nick = taken.Nick; return a }},
	} {
		t.Run(tc.constraint, func(t *testing.T) {
			store := memory.New()
			ctx := withExecution(t, context.Background())
			seedAccount(t, store, ctx, "a-1", newAccount(1))

			var saved error
			err := withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
				saved = res.Accounts.Save(ctx, "a-2", tc.collide(newAccount(1), newAccount(2)), 0)
				return nil
			})

			if err != nil {
				t.Fatalf("Within() = %v, want nil", err)
			}
			requireViolation(t, saved, tc.constraint, tc.rejection)
			if _, _, err := accounts.Reader(store).Load(ctx, "a-2"); !errors.Is(err, ports.ErrNotFound) {
				t.Fatalf("Load(a-2) = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestAnUpdateToATakenValueIsRefusedAndKeepsTheVersion(t *testing.T) {
	store := memory.New()
	ctx := withExecution(t, context.Background())
	seedAccount(t, store, ctx, "a-1", newAccount(1))
	seedAccount(t, store, ctx, "a-2", newAccount(2))

	var saved error
	_ = withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
		moved := newAccount(3)
		moved.Email = newAccount(1).Email
		saved = res.Accounts.Save(ctx, "a-2", moved, 1)
		return nil
	})

	requireViolation(t, saved, emailKey, emailTaken)
	if got, version, err := accounts.Reader(store).Load(ctx, "a-2"); err != nil || version != 1 || got.Marker != 2 {
		t.Fatalf("Load(a-2) = (marker %d, v%d, %v), want (2, v1, nil)", got.Marker, version, err)
	}
}

func TestResavingTheSameIdentityWithItsOwnValuesIsNotAViolation(t *testing.T) {
	store := memory.New()
	ctx := withExecution(t, context.Background())
	seedAccount(t, store, ctx, "a-1", newAccount(1))

	err := withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
		same := newAccount(1)
		same.Marker = 9
		return res.Accounts.Save(ctx, "a-1", same, 1)
	})

	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}
}

func TestTheSameValueUnderAnotherTenantIsWritten(t *testing.T) {
	store := memory.New()
	seedAccount(t, store, withExecution(t, context.Background()), "a-1", newAccount(1))
	globex := execctx.WithExecution(t, context.Background(), execctx.WithTenant("globex"))

	err := withinAccounts(t, store, globex, func(ctx context.Context, res accountResources) error {
		return res.Accounts.Save(ctx, "a-2", newAccount(1), 0)
	})

	if err != nil {
		t.Fatalf("Within() under another tenant = %v, want nil", err)
	}
}

func TestANullComponentCollidesWithNothing(t *testing.T) {
	store := memory.New()
	ctx := withExecution(t, context.Background())
	first := newAccount(1)
	first.Nick, first.NickNull = "", true
	seedAccount(t, store, ctx, "a-1", first)

	err := withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
		second := newAccount(2)
		second.Nick, second.NickNull = "", true
		return res.Accounts.Save(ctx, "a-2", second, 0)
	})

	if err != nil {
		t.Fatalf("Within() = %v, want nil: NULL never collides", err)
	}
}

func TestAnEmptyValueCollidesLikeAnyOther(t *testing.T) {
	store := memory.New()
	ctx := withExecution(t, context.Background())
	first := newAccount(1)
	first.Nick = ""
	seedAccount(t, store, ctx, "a-1", first)

	var saved error
	_ = withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
		second := newAccount(2)
		second.Nick = ""
		saved = res.Accounts.Save(ctx, "a-2", second, 0)
		return nil
	})

	requireViolation(t, saved, nickKey, nickTaken)
}

func TestTheComponentsOfAKeyAreComparedOneByOne(t *testing.T) {
	store := memory.New()
	ctx := withExecution(t, context.Background())
	first := newAccount(1)
	first.Org, first.Handle = "ab", "c"
	seedAccount(t, store, ctx, "a-1", first)

	err := withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
		second := newAccount(2)
		second.Org, second.Handle = "a", "bc"
		return res.Accounts.Save(ctx, "a-2", second, 0)
	})

	if err != nil {
		t.Fatalf("Within() = %v, want nil: (ab, c) and (a, bc) are two keys", err)
	}
}

func TestAPriorWriteLeavesTheViolationWithoutRejection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(ctx context.Context, res accountResources) error
	}{
		{"save", func(ctx context.Context, res accountResources) error {
			return res.Accounts.Save(ctx, "a-3", newAccount(3), 0)
		}},
		{"enqueue", func(ctx context.Context, res accountResources) error {
			return res.Outbox.Enqueue(ctx, entry("m-1"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := memory.New()
			ctx := withExecution(t, context.Background())
			seedAccount(t, store, ctx, "a-1", newAccount(1))

			var saved error
			_ = withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
				if err := tc.write(ctx, res); err != nil {
					return err
				}
				collides := newAccount(2)
				collides.Email = newAccount(1).Email
				saved = res.Accounts.Save(ctx, "a-2", collides, 0)
				return nil
			})

			requireViolation(t, saved, emailKey, nil)
		})
	}
}

func TestAnInboxRegistrationIsNotAPriorWrite(t *testing.T) {
	store := memory.New()
	ctx := withExecution(t, context.Background())
	seedAccount(t, store, ctx, "a-1", newAccount(1))

	var saved error
	err := withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
		if _, err := res.Inbox.Register(ctx, ports.Receipt{Consumer: "accounts-consumer", MessageID: "m-1", PayloadHash: "h"}); err != nil {
			return err
		}
		collides := newAccount(2)
		collides.Email = newAccount(1).Email
		saved = res.Accounts.Save(ctx, "a-2", collides, 0)
		return nil
	})

	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}
	requireViolation(t, saved, emailKey, emailTaken)
}

func TestTheTransactionStaysUsableAfterAViolation(t *testing.T) {
	store := memory.New()
	ctx := withExecution(t, context.Background())
	seedAccount(t, store, ctx, "a-1", newAccount(1))

	var after error
	err := withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
		_ = res.Accounts.Save(ctx, "a-2", newAccount(1), 0)
		after = res.Accounts.Save(ctx, "a-3", newAccount(3), 0)
		return nil
	})

	if err != nil || after != nil {
		t.Fatalf("Within() = %v, Save after the violation = %v; want nil, nil", err, after)
	}
	if _, _, err := accounts.Reader(store).Load(ctx, "a-3"); err != nil {
		t.Fatalf("Load(a-3) = %v, want nil", err)
	}
}

func TestAStaleVersionIsAConflictBeforeItIsAViolation(t *testing.T) {
	store := memory.New()
	ctx := withExecution(t, context.Background())
	seedAccount(t, store, ctx, "a-1", newAccount(1))
	seedAccount(t, store, ctx, "a-2", newAccount(2))

	err := withinAccounts(t, store, ctx, func(ctx context.Context, res accountResources) error {
		return res.Accounts.Save(ctx, "a-2", newAccount(1), 7)
	})

	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Save() at a stale version onto a taken value = %v, want ErrVersionConflict", err)
	}
}

func TestARepositoryOverAnInvalidDeclarationPanics(t *testing.T) {
	key := func(a account) ([]string, bool) { return []string{a.Email}, true }
	for _, tc := range []struct {
		name   string
		unique []memory.Unique[account]
		want   string
	}{
		{"unnamed", []memory.Unique[account]{{Key: key, Rejection: emailTaken}}, "without a name"},
		{"keyless", []memory.Unique[account]{{Name: emailKey, Rejection: emailTaken}}, "without a key"},
		{"undeclared rejection", []memory.Unique[account]{{Name: emailKey, Key: key}}, "without a rejection"},
		{"invalid code", []memory.Unique[account]{{Name: emailKey, Key: key, Rejection: domain.Reject("Not A Code", "x")}}, "invalid rejection code"},
		{"repeated name", []memory.Unique[account]{
			{Name: emailKey, Key: key, Rejection: emailTaken},
			{Name: emailKey, Key: key, Rejection: handleTaken},
		}, "declared twice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				got, _ := recover().(string)
				if !strings.Contains(got, tc.want) {
					t.Fatalf("panic = %q, want it to name %q", got, tc.want)
				}
			}()
			table := memory.Table[string, account]{Name: "accounts", Unique: tc.unique}
			table.Repository(nil)
		})
	}
}
