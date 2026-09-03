package memory_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/memory"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// resources is what the composition root binds to an open transaction. It
// lives in the test because provider → application is a forbidden cell: the
// realization cannot know the use case's resource type.
type resources struct {
	Orders dmpfports.Repository[orders.OrderID, orders.Snapshot]
	Outbox dmpfports.Outbox
}

func bind(tx *memory.Tx) resources {
	return resources{Orders: tx.Orders(), Outbox: tx.Outbox()}
}

const orderID = orders.OrderID("P-100")

var errBoom = errors.New("memory_test: boom")

func openSnapshot(quantity int) orders.Snapshot {
	return orders.Snapshot{
		ID:        orderID,
		Status:    orders.Open,
		ItemLimit: 3,
		Items:     []orders.Item{{SKU: "ABC", Quantity: quantity}},
	}
}

func entry(id dmpfports.MessageID) dmpfports.OutboxEntry {
	return dmpfports.OutboxEntry{
		MessageID:        id,
		OccurredAt:       1_755_432_000_000_000_000,
		Intent:           dmpfports.PublishIntent{Destination: "orders.events", PartitionKey: string(orderID)},
		AggregateType:    "orders.Order",
		AggregateID:      string(orderID),
		AggregateVersion: 1,
		Event:            orders.ItemAdded{Order: orderID, SKU: "ABC", Quantity: 1, At: 1_755_432_000},
	}
}

func seed(t *testing.T, store *memory.Store, snapshot orders.Snapshot, expected dmpfports.Version) {
	t.Helper()
	uow := memory.NewUnitOfWork(store, bind)
	err := uow.Within(context.Background(), func(ctx context.Context, res resources) error {
		return res.Orders.Save(ctx, snapshot.ID, snapshot, expected)
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestCommitAppliesBothWritesToTheStore(t *testing.T) {
	store := memory.New()
	uow := memory.NewUnitOfWork(store, bind)

	err := uow.Within(context.Background(), func(ctx context.Context, res resources) error {
		if err := res.Orders.Save(ctx, orderID, openSnapshot(1), 0); err != nil {
			return err
		}
		return res.Outbox.Enqueue(ctx, entry("m-000001"))
	})
	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	snapshot, version, err := store.Reader().Load(context.Background(), orderID)
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
	if !snapshot.Equal(openSnapshot(1)) {
		t.Fatalf("snapshot = %+v, want %+v", snapshot, openSnapshot(1))
	}
	if got := store.Entries(); len(got) != 1 || got[0].MessageID != "m-000001" {
		t.Fatalf("Entries() = %+v, want exactly one entry m-000001", got)
	}
}

func TestAFailingCallbackKeepsNothing(t *testing.T) {
	store := memory.New()
	uow := memory.NewUnitOfWork(store, bind)

	err := uow.Within(context.Background(), func(ctx context.Context, res resources) error {
		if err := res.Orders.Save(ctx, orderID, openSnapshot(1), 0); err != nil {
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

	err := uow.Within(context.Background(), func(ctx context.Context, res resources) error {
		if err := res.Orders.Save(ctx, orderID, openSnapshot(1), 0); err != nil {
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

	if err := uow.Within(context.Background(), noop); !errors.Is(err, errBoom) {
		t.Fatalf("first Within() = %v, want errBoom", err)
	}
	if err := uow.Within(context.Background(), noop); err != nil {
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

		_ = uow.Within(context.Background(), func(ctx context.Context, res resources) error {
			if err := res.Orders.Save(ctx, orderID, openSnapshot(1), 0); err != nil {
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
	seed(t, store, openSnapshot(1), 0)

	loaded, _, err := store.Reader().Load(context.Background(), orderID)
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	loaded.Items[0].Quantity = 999

	again, _, _ := store.Reader().Load(context.Background(), orderID)
	if again.Items[0].Quantity != 1 {
		t.Fatalf("mutating a loaded snapshot reached the store: Quantity = %d, want 1", again.Items[0].Quantity)
	}
}

func TestSaveSharesNoSliceWithTheCaller(t *testing.T) {
	store := memory.New()
	snapshot := openSnapshot(1)
	seed(t, store, snapshot, 0)

	snapshot.Items[0].Quantity = 999

	loaded, _, _ := store.Reader().Load(context.Background(), orderID)
	if loaded.Items[0].Quantity != 1 {
		t.Fatalf("mutating the caller's slice reached the store: Quantity = %d, want 1", loaded.Items[0].Quantity)
	}
}

// TestReaderInsideWithinReadsTheCommittedState pins the reason Store keeps two
// mutexes: with a single one serializing transactions and guarding the state,
// this read would deadlock instead of returning.
func TestReaderInsideWithinReadsTheCommittedState(t *testing.T) {
	store := memory.New()
	seed(t, store, openSnapshot(1), 0)
	uow := memory.NewUnitOfWork(store, bind)

	var seenQuantity int
	var seenVersion dmpfports.Version

	err := uow.Within(context.Background(), func(ctx context.Context, res resources) error {
		if err := res.Orders.Save(ctx, orderID, openSnapshot(7), 1); err != nil {
			return err
		}
		snapshot, version, err := store.Reader().Load(ctx, orderID)
		if err != nil {
			return err
		}
		seenQuantity = snapshot.Items[0].Quantity
		seenVersion = version
		return nil
	})
	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	if seenQuantity != 1 || seenVersion != 1 {
		t.Fatalf("Reader() inside the callback saw quantity %d at v%d, want the uncommitted 1 at v1",
			seenQuantity, seenVersion)
	}

	after, version, _ := store.Reader().Load(context.Background(), orderID)
	if after.Items[0].Quantity != 7 || version != 2 {
		t.Fatalf("after the commit: quantity %d at v%d, want 7 at v2", after.Items[0].Quantity, version)
	}
}

// Without the clone in commit, a port kept past the callback would write
// straight into the committed state, outside any transaction and without dataMu.
func TestAPortThatEscapesTheCallbackCannotReachTheStore(t *testing.T) {
	store := memory.New()
	uow := memory.NewUnitOfWork(store, bind)

	var escaped resources
	err := uow.Within(context.Background(), func(ctx context.Context, res resources) error {
		escaped = res
		return res.Orders.Save(ctx, orderID, openSnapshot(1), 0)
	})
	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	if err := escaped.Orders.Save(context.Background(), orderID, openSnapshot(9), 1); err != nil {
		t.Fatalf("Save through the escaped port = %v, want nil (it writes the discarded copy)", err)
	}
	if err := escaped.Outbox.Enqueue(context.Background(), entry("m-999999")); err != nil {
		t.Fatalf("Enqueue through the escaped port = %v, want nil", err)
	}

	snapshot, version, _ := store.Reader().Load(context.Background(), orderID)
	if snapshot.Items[0].Quantity != 1 || version != 1 {
		t.Fatalf("the escaped port reached the store: quantity %d at v%d, want 1 at v1",
			snapshot.Items[0].Quantity, version)
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
	ctx, cancel := context.WithCancel(context.Background())

	holding := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = uow.Within(context.Background(), func(context.Context, resources) error {
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

	cancel()
	close(release)

	if err := <-queued; !errors.Is(err, context.Canceled) {
		t.Fatalf("the queued Within() = %v, want context.Canceled", err)
	}
	if got := store.Commits(); got != 1 {
		t.Fatalf("Commits() = %d, want 1 — only the first transaction committed", got)
	}
}

func TestSaveRejectsADivergentExpectedVersion(t *testing.T) {
	store := memory.New()
	seed(t, store, openSnapshot(1), 0)
	uow := memory.NewUnitOfWork(store, bind)

	err := uow.Within(context.Background(), func(ctx context.Context, res resources) error {
		return res.Orders.Save(ctx, orderID, openSnapshot(2), 0)
	})

	if !errors.Is(err, dmpfports.ErrVersionConflict) {
		t.Fatalf("Within() = %v, want ErrVersionConflict", err)
	}
	loaded, version, _ := store.Reader().Load(context.Background(), orderID)
	if version != 1 || loaded.Items[0].Quantity != 1 {
		t.Fatalf("the store changed under a conflict: version = %d, quantity = %d", version, loaded.Items[0].Quantity)
	}
}

func TestLoadReportsErrNotFoundForAnAbsentAggregate(t *testing.T) {
	store := memory.New()

	_, _, err := store.Reader().Load(context.Background(), "P-404")

	if !errors.Is(err, dmpfports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound", err)
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
			_ = uow.Within(context.Background(), func(ctx context.Context, res resources) error {
				return res.Outbox.Enqueue(ctx, entry(dmpfports.MessageID("m-"+string(rune('a'+i)))))
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
	if _, _, err := store.Reader().Load(context.Background(), orderID); !errors.Is(err, dmpfports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound — the store must be untouched", err)
	}
	if got := store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
}
