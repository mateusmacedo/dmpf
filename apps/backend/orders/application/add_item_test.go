package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var errCommitFailed = errors.New("application_test: commit failed")

func TestAddItemCreatesTheAggregateAndAuthorsTheOutboxEntry(t *testing.T) {
	h := newHarness(t)

	out, err := h.service.AddItem(context.Background(), testExecution(t), application.AddItem{Order: orderID, SKU: "ABC", Quantity: 1})

	if err != nil {
		t.Fatalf("AddItem() error = %v, want nil", err)
	}
	if rej, refused := out.Rejection(); refused {
		t.Fatalf("Rejection() = %v, want no refusal", rej)
	}
	if got, want := out.Response(), (domain.ItemAccepted{Order: orderID, Items: 1}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}

	snapshot, version, err := ordersTable.Reader(h.store).Load(context.Background(), orderID)
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("snapshot has %d items, want 1", len(snapshot.Items))
	}

	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() has %d entries, want 1", len(entries))
	}
	got := entries[0]
	want := ports.OutboxEntry{
		MessageID:        "m-000001",
		OccurredAt:       occurred,
		Intent:           ports.PublishIntent{Destination: application.Destination, PartitionKey: string(orderID)},
		AggregateType:    application.AggregateType,
		AggregateID:      string(orderID),
		AggregateVersion: 1,
		Event:            domain.ItemAdded{Order: orderID, SKU: "ABC", Quantity: 1, At: domain.Instant(occurred.Unix())},
		Context:          ports.MessageContext{CausationID: "m-000001"},
	}
	if got != want {
		t.Fatalf("OutboxEntry mismatch\ngot:  %+v\nwant: %+v", got, want)
	}
	if got.Event.EventName() != "orders.item-added" {
		t.Fatalf("EventName() = %q, want %q", got.Event.EventName(), "orders.item-added")
	}
}

// The adapter authors correlation and trace at the edge (FND-07 §8.6 item 3);
// the service copies them and, as the origin of the chain, names itself as the
// cause (FND-05 ENV-08).
func TestAddItemCopiesTheMessageContextIntoTheOutboxEntry(t *testing.T) {
	h := newHarness(t)
	const traceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	ctx := ports.WithMessageContext(context.Background(), ports.MessageContext{CorrelationID: "corr-1", Traceparent: traceparent})

	if _, err := h.service.AddItem(ctx, testExecution(t), application.AddItem{Order: orderID, SKU: "ABC", Quantity: 1}); err != nil {
		t.Fatalf("AddItem() error = %v, want nil", err)
	}

	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() has %d entries, want 1", len(entries))
	}
	want := ports.MessageContext{CorrelationID: "corr-1", CausationID: "m-000001", Traceparent: traceparent}
	if entries[0].Context != want {
		t.Fatalf("Context = %+v, want %+v", entries[0].Context, want)
	}
}

func TestAddItemLoadsAnExistingAggregate(t *testing.T) {
	h := newHarness(t)
	h.seed(t, openSnapshot(1), 0)

	out, err := h.service.AddItem(context.Background(), testExecution(t), application.AddItem{Order: orderID, SKU: "XYZ", Quantity: 2})

	if err != nil {
		t.Fatalf("AddItem() error = %v, want nil", err)
	}
	if got, want := out.Response(), (domain.ItemAccepted{Order: orderID, Items: 2}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	snapshot, version, _ := ordersTable.Reader(h.store).Load(context.Background(), orderID)
	if version != 2 {
		t.Fatalf("version = %d, want 2 — the stored version plus one", version)
	}
	if len(snapshot.Items) != 2 {
		t.Fatalf("snapshot has %d items, want 2", len(snapshot.Items))
	}
	if got := h.store.Entries()[0].AggregateVersion; got != 2 {
		t.Fatalf("AggregateVersion = %d, want 2 — the version actually written", got)
	}
}

func TestAddItemRejectedCommitsWithoutWriting(t *testing.T) {
	h := newHarness(t)
	h.seed(t, openSnapshot(itemLimit), 0)
	before, versionBefore, _ := ordersTable.Reader(h.store).Load(context.Background(), orderID)

	out, err := h.service.AddItem(context.Background(), testExecution(t), application.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if err != nil {
		t.Fatalf("AddItem() error = %v, want nil — a refusal is not a technical failure (DEC-04)", err)
	}
	rej, refused := out.Rejection()
	if !refused {
		t.Fatal("Rejection() reported no refusal, want orders/item-limit-exceeded")
	}
	if rej.Code() != domain.CodeItemLimitExceeded {
		t.Fatalf("Code() = %q, want %q", rej.Code(), domain.CodeItemLimitExceeded)
	}
	if got := out.Response(); got != (domain.ItemAccepted{}) {
		t.Fatalf("Response() = %+v, want the zero response", got)
	}

	after, versionAfter, _ := ordersTable.Reader(h.store).Load(context.Background(), orderID)
	if !after.Equal(before) || versionAfter != versionBefore {
		t.Fatalf("the store changed under a refusal: %+v v%d, want %+v v%d", after, versionAfter, before, versionBefore)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
	if got := h.serviceWithinCalls(); got != 1 {
		t.Fatalf("transactions opened = %d, want 1", got)
	}
	if got := h.serviceCommits(); got != 1 {
		t.Fatalf("commits = %d, want 1 — the commit occurs under a refusal (UOW-05, UOW-06)", got)
	}
	if h.saves != 0 || h.enqueues != 0 {
		t.Fatalf("a refusal wrote: saves = %d, enqueues = %d, want 0 and 0", h.saves, h.enqueues)
	}
}

func TestAddItemKeepsNothingWhenTheCommitFailsWhileCreating(t *testing.T) {
	h := newHarness(t)
	h.store.FailNextCommit(errCommitFailed)

	out, err := h.service.AddItem(context.Background(), testExecution(t), application.AddItem{Order: orderID, SKU: "ABC", Quantity: 1})

	if !errors.Is(err, errCommitFailed) {
		t.Fatalf("AddItem() error = %v, want errCommitFailed", err)
	}
	if got := out.Response(); got != (domain.ItemAccepted{}) {
		t.Fatalf("Response() = %+v, want the zero outcome", got)
	}
	if _, _, err := ordersTable.Reader(h.store).Load(context.Background(), orderID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound — a failed commit creates nothing (UOW-07)", err)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
}

func TestAddItemUnderAVersionConflictStopsBeforeTheOutbox(t *testing.T) {
	h := newHarness(t, withSaveError(ports.ErrVersionConflict))
	h.seed(t, openSnapshot(1), 0)
	h.seed(t, openSnapshot(1), 1)

	_, err := h.service.AddItem(context.Background(), testExecution(t), application.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("AddItem() error = %v, want ErrVersionConflict", err)
	}
	if h.binds != 1 {
		t.Fatalf("the callback received resources %d times, want 1 — Within never repeats it (UOW-09)", h.binds)
	}
	if h.saves != 1 {
		t.Fatalf("Save called %d times, want 1 — no retry (UOW-10)", h.saves)
	}
	if h.enqueues != 0 {
		t.Fatalf("Enqueue called %d times, want 0 — the conflict stops before any outbox record", h.enqueues)
	}
	if got := h.store.Entries(); len(got) != 0 {
		t.Fatalf("Entries() = %+v, want empty", got)
	}
	if _, version, _ := ordersTable.Reader(h.store).Load(context.Background(), orderID); version != 2 {
		t.Fatalf("version = %d, want 2 — the stored aggregate is untouched", version)
	}
}

// TestWritingIntoAnUnboundResourceEscapesTheTransaction is UOW-04 seen from the
// outside: a port the callback was not given is not inside the boundary, so its
// write survives the rollback of the transaction that wrapped it.
func TestWritingIntoAnUnboundResourceEscapesTheTransaction(t *testing.T) {
	a, b := memory.New(), memory.New()
	bind := func(tx *memory.Tx) application.Resources {
		return application.Resources{Orders: ordersTable.Repository(tx), Outbox: tx.Outbox()}
	}
	unbound := memory.NewUnitOfWork(b, bind)
	const escaped = domain.OrderID("P-300")

	err := memory.NewUnitOfWork(a, bind).Within(context.Background(),
		func(ctx context.Context, res application.Resources) error {
			if err := res.Orders.Save(ctx, orderID, openSnapshot(1), 0); err != nil {
				return err
			}
			writeErr := unbound.Within(ctx, func(ctx context.Context, res application.Resources) error {
				snapshot := openSnapshot(1)
				snapshot.ID = escaped
				return res.Orders.Save(ctx, escaped, snapshot, 0)
			})
			if writeErr != nil {
				return writeErr
			}
			return errCommitFailed
		})

	if !errors.Is(err, errCommitFailed) {
		t.Fatalf("Within() = %v, want errCommitFailed", err)
	}
	if _, _, err := ordersTable.Reader(a).Load(context.Background(), orderID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("store A kept a write after rollback: Load() = %v, want ErrNotFound", err)
	}
	if _, version, err := ordersTable.Reader(b).Load(context.Background(), escaped); err != nil || version != 1 {
		t.Fatalf("store B lost the write it committed on its own: err = %v, version = %d", err, version)
	}
}
