package application_test

import (
	"context"
	"errors"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

var reservationsTable = memory.Table[domain.OrderID, domain.Snapshot]{Name: "reservations"}

const consumer = "reservations"

func bind(tx *memory.Tx) application.Resources {
	return application.Resources{
		Inbox:        tx.Inbox(consumer),
		Reservations: reservationsTable.Repository(tx),
		Outbox:       tx.Outbox(),
	}
}

func newService(store *memory.Store) application.Service {
	return application.Service{
		UoW:       memory.NewUnitOfWork(store, bind),
		Clock:     memory.FixedClock{At: 1_755_432_000_000_000_000},
		IDs:       &memory.SequenceIDs{Prefix: "m-"},
		Authorize: usecase.AllowAll[application.Operation](),
		Consumer:  consumer,
	}
}

func consumeOrderPlaced(id ports.MessageID, hash string, order domain.OrderID, items int) application.ConsumeOrderPlaced {
	return application.ConsumeOrderPlaced{
		MessageID: id, MessageType: "orders.order-placed", PayloadHash: hash, ReceivedAt: 1_755_431_000_000_000_000,
		Order: order, Items: items,
	}
}

// failingReservations wraps a real repository and fails every Save with
// failure, which is how tests 3 and 4 reach D3/D4 without a broken aggregate.
type failingReservations struct {
	ports.Repository[domain.OrderID, domain.Snapshot]
	failure error
}

func (f failingReservations) Save(context.Context, domain.OrderID, domain.Snapshot, ports.Version) error {
	return f.failure
}

func newServiceWithFailingSave(store *memory.Store, failure error) application.Service {
	svc := newService(store)
	svc.UoW = memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
		res := bind(tx)
		res.Reservations = failingReservations{Repository: res.Reservations, failure: failure}
		return res
	})
	return svc
}

// neverCompletingInbox always returns a first reception whose Pending never
// reports itself completed — the defect achado 3 of the external review covers.
type neverCompletingInbox struct{}

func (neverCompletingInbox) Register(context.Context, ports.Receipt) (ports.Reception, error) {
	return ports.FirstReception(brokenPending{}), nil
}

type brokenPending struct{}

func (brokenPending) Complete(context.Context, ports.Completion) error { return nil }
func (brokenPending) Completed() bool                                  { return false }

func newServiceWithBrokenInbox(store *memory.Store) application.Service {
	svc := newService(store)
	svc.UoW = memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
		res := bind(tx)
		res.Inbox = neverCompletingInbox{}
		return res
	})
	return svc
}

func requireNothingPersisted(t *testing.T, store *memory.Store) {
	t.Helper()
	if got := store.InboxRows(); got != 0 {
		t.Fatalf("InboxRows() = %d, want 0", got)
	}
	if got := len(store.Entries()); got != 0 {
		t.Fatalf("Entries() has %d elements, want 0", got)
	}
}

func TestConsumeFirstReceptionAppliesAndConfirms(t *testing.T) {
	store := memory.New()
	svc := newService(store)

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3))

	if err != nil {
		t.Fatalf("Consume() error = %v, want nil", err)
	}
	if disp != usecase.R1D1 {
		t.Fatalf("Consume() disposition = %v, want %v", disp, usecase.R1D1)
	}
	snapshot, _, err := reservationsTable.Reader(store).Load(withExecution(t, context.Background()), "P-100")
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if snapshot.Status != domain.Confirmed || snapshot.Items != 3 {
		t.Fatalf("Snapshot = %+v, want Confirmed with 3 items", snapshot)
	}
	entries := store.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() has %d elements, want 1", len(entries))
	}
	if _, ok := entries[0].Event.(domain.ReservationConfirmed); !ok {
		t.Fatalf("Entries()[0].Event = %T, want ReservationConfirmed", entries[0].Event)
	}
	status, ok := store.InboxStatus(consumer, "m-ext-1")
	if !ok || status != ports.StatusProcessed {
		t.Fatalf("InboxStatus() = (%v, %v), want (processed, true)", status, ok)
	}
}

// A consumer continues a chain: the adapter authored the consumed message as
// the cause (FND-07 §8.6 item 3) and the service copies it as is; only when
// nobody authored anything does the fact name itself (FND-05 ENV-08).
func TestConsumeCopiesTheMessageContextIntoTheOutboxEntry(t *testing.T) {
	const traceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	tests := []struct {
		name string
		ctx  context.Context
		want ports.MessageContext
	}{
		{
			name: "authored by the adapter",
			ctx:  ports.WithMessageContext(context.Background(), ports.MessageContext{CorrelationID: "corr-1", CausationID: "m-ext-1", Traceparent: traceparent}),
			want: ports.MessageContext{CorrelationID: "corr-1", CausationID: "m-ext-1", Traceparent: traceparent},
		},
		{
			name: "nothing authored",
			ctx:  context.Background(),
			want: ports.MessageContext{CausationID: "m-000001"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := memory.New()
			svc := newService(store)

			if _, err := svc.Consume(withExecution(t, tt.ctx), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3)); err != nil {
				t.Fatalf("Consume() error = %v, want nil", err)
			}

			entries := store.Entries()
			if len(entries) != 1 {
				t.Fatalf("Entries() has %d elements, want 1", len(entries))
			}
			if entries[0].Context != tt.want {
				t.Fatalf("Context = %+v, want %+v", entries[0].Context, tt.want)
			}
		})
	}
}

func TestConsumeZeroItemsRejects(t *testing.T) {
	store := memory.New()
	svc := newService(store)

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 0))

	if err != nil {
		t.Fatalf("Consume() error = %v, want nil", err)
	}
	if disp != usecase.R1D2 {
		t.Fatalf("Consume() disposition = %v, want %v", disp, usecase.R1D2)
	}
	status, ok := store.InboxStatus(consumer, "m-ext-1")
	if !ok || status != ports.StatusRejected {
		t.Fatalf("InboxStatus() = (%v, %v), want (rejected, true)", status, ok)
	}
	if got, _ := store.InboxLastError(consumer, "m-ext-1"); got != string(domain.CodeNothingToReserve) {
		t.Fatalf("InboxLastError() = %q, want %q", got, domain.CodeNothingToReserve)
	}
	if got := len(store.Entries()); got != 0 {
		t.Fatalf("Entries() has %d elements, want 0", got)
	}
	if _, _, err := reservationsTable.Reader(store).Load(withExecution(t, context.Background()), "P-100"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound — a rejection persists no reservation", err)
	}
}

func TestConsumeATransientSaveFailureIsD3AndPersistsNothing(t *testing.T) {
	store := memory.New()
	failure := usecase.NewFailure(usecase.TransientDependency, true, errors.New("consume_test: dependency down"))
	svc := newServiceWithFailingSave(store, failure)

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3))

	if !errors.Is(err, failure) {
		t.Fatalf("Consume() error = %v, want the injected failure", err)
	}
	if disp != usecase.R1D3 {
		t.Fatalf("Consume() disposition = %v, want %v", disp, usecase.R1D3)
	}
	requireNothingPersisted(t, store)
}

func TestConsumeATerminalSaveFailureIsD4AndPersistsNothing(t *testing.T) {
	store := memory.New()
	failure := usecase.NewFailure(usecase.Unexpected, false, errors.New("consume_test: boom"))
	svc := newServiceWithFailingSave(store, failure)

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3))

	if !errors.Is(err, failure) {
		t.Fatalf("Consume() error = %v, want the injected failure", err)
	}
	if disp != usecase.R1D4 {
		t.Fatalf("Consume() disposition = %v, want %v", disp, usecase.R1D4)
	}
	requireNothingPersisted(t, store)
}

func TestConsumeRedeliveryOfAProcessedMessageIsR2(t *testing.T) {
	store := memory.New()
	svc := newService(store)
	if _, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3)); err != nil {
		t.Fatalf("setup Consume() = %v, want nil", err)
	}

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3))

	if err != nil {
		t.Fatalf("Consume() error = %v, want nil", err)
	}
	if disp != usecase.R2 {
		t.Fatalf("Consume() disposition = %v, want %v", disp, usecase.R2)
	}
	if got := len(store.Entries()); got != 1 {
		t.Fatalf("Entries() has %d elements, want 1 — no new outbox write", got)
	}
}

func TestConsumeRedeliveryOfARejectedMessageIsR3(t *testing.T) {
	store := memory.New()
	svc := newService(store)
	if _, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 0)); err != nil {
		t.Fatalf("setup Consume() = %v, want nil", err)
	}

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 0))

	if err != nil {
		t.Fatalf("Consume() error = %v, want nil", err)
	}
	if disp != usecase.R3 {
		t.Fatalf("Consume() disposition = %v, want %v", disp, usecase.R3)
	}
	if got := len(store.Entries()); got != 0 {
		t.Fatalf("Entries() has %d elements, want 0 (INB-12): a rejection never reemits", got)
	}
}

func TestConsumeADivergentHashOnAPresentKeyIsR4(t *testing.T) {
	store := memory.New()
	svc := newService(store)
	if _, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3)); err != nil {
		t.Fatalf("setup Consume() = %v, want nil", err)
	}

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h2", "P-100", 3))

	if err != nil {
		t.Fatalf("Consume() error = %v, want nil", err)
	}
	if disp != usecase.R4 {
		t.Fatalf("Consume() disposition = %v, want %v", disp, usecase.R4)
	}
	if got := len(store.Entries()); got != 1 {
		t.Fatalf("Entries() has %d elements, want 1 — nothing written by the collision", got)
	}
}

func TestConsumeAFailedCommitClassifiesToR1D4AndPersistsNothing(t *testing.T) {
	store := memory.New()
	store.FailNextCommit(errors.New("consume_test: commit refused"))
	svc := newService(store)

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3))

	if err == nil {
		t.Fatal("Consume() error = nil, want the commit failure")
	}
	if disp != usecase.R1D4 {
		t.Fatalf("Consume() disposition = %v, want %v (INB-07)", disp, usecase.R1D4)
	}
	requireNothingPersisted(t, store)
}

func TestConsumeAPendingLeftUncompletedIsR1D4AndPersistsNothing(t *testing.T) {
	store := memory.New()
	svc := newServiceWithBrokenInbox(store)

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3))

	if !errors.Is(err, ports.ErrPendingNotCompleted) {
		t.Fatalf("Consume() error = %v, want ErrPendingNotCompleted", err)
	}
	if disp != usecase.R1D4 {
		t.Fatalf("Consume() disposition = %v, want %v", disp, usecase.R1D4)
	}
	requireNothingPersisted(t, store)
}

func TestConsumeTwoMessagesForTheSameOrderReserveOnlyOnce(t *testing.T) {
	store := memory.New()
	svc := newService(store)
	if _, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3)); err != nil {
		t.Fatalf("first Consume() = %v, want nil", err)
	}

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-2", "h2", "P-100", 5))

	if err != nil {
		t.Fatalf("second Consume() error = %v, want nil", err)
	}
	if disp != usecase.R1D2 {
		t.Fatalf("second Consume() disposition = %v, want %v (GAR-10)", disp, usecase.R1D2)
	}
	if got, _ := store.InboxLastError(consumer, "m-ext-2"); got != string(domain.CodeAlreadyReserved) {
		t.Fatalf("InboxLastError() = %q, want %q", got, domain.CodeAlreadyReserved)
	}
	snapshot, _, err := reservationsTable.Reader(store).Load(withExecution(t, context.Background()), "P-100")
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if snapshot.Items != 3 {
		t.Fatalf("Snapshot.Items = %d, want 3 — the second command must not have reserved again", snapshot.Items)
	}
}

func TestConsumeAuthorizeDenyingNeverOpensATransaction(t *testing.T) {
	store := memory.New()
	svc := newService(store)
	svc.Authorize = func(context.Context, application.Operation) error {
		return errors.New("consume_test: not authorized")
	}

	disposition, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-1", "h1", "P-100", 3))

	if err == nil {
		t.Fatal("Consume() error = nil, want the authorization failure")
	}
	// A refusal without category is Unexpected and terminal (ERR-11): the
	// adapter must receive one of the seven, never the zero value.
	if disposition != usecase.R1D4 {
		t.Fatalf("disposition = %v, want %v", disposition, usecase.R1D4)
	}
	if got := store.WithinCalls(); got != 0 {
		t.Fatalf("WithinCalls() = %d, want 0", got)
	}
}

func TestConsumeAuthorizeDenyingWithACategoryKeepsIt(t *testing.T) {
	store := memory.New()
	svc := newService(store)
	svc.Authorize = func(context.Context, application.Operation) error {
		return usecase.NewFailure(usecase.Forbidden, false, errors.New("consume_test: forbidden"))
	}

	disposition, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("m-ext-2", "h1", "P-100", 3))

	var failure *usecase.Failure
	if !errors.As(err, &failure) || failure.Category() != usecase.Forbidden {
		t.Fatalf("err = %v, want the Forbidden failure", err)
	}
	if disposition != usecase.R1D4 {
		t.Fatalf("disposition = %v, want %v (FND-07 §6.2: Forbidden is terminal)", disposition, usecase.R1D4)
	}
}

func TestConsumeOnACanceledReservationIsR1D2WithoutWriting(t *testing.T) {
	store := memory.New()
	svc := newService(store)
	if _, err := svc.Cancel(withExecution(t, context.Background()), application.Cancel{Order: "o-1"}); err != nil {
		t.Fatalf("setup: Cancel() error = %v", err)
	}
	entriesBefore := len(store.Entries())

	disp, err := svc.Consume(withExecution(t, context.Background()), consumeOrderPlaced("msg-1", "hash-1", "o-1", 2))

	if err != nil {
		t.Fatalf("Consume() error = %v, want nil — a refusal is not a technical failure", err)
	}
	if disp != usecase.R1D2 {
		t.Fatalf("disposition = %v, want R1D2 — the cancellation decided first", disp)
	}
	if status, _ := store.InboxStatus(consumer, "msg-1"); status != ports.StatusRejected {
		t.Fatalf("inbox status = %v, want rejected", status)
	}
	if last, _ := store.InboxLastError(consumer, "msg-1"); last != string(domain.CodeReservationCanceled) {
		t.Fatalf("inbox last error = %q, want %q", last, domain.CodeReservationCanceled)
	}
	if got := len(store.Entries()); got != entriesBefore {
		t.Fatalf("Entries() = %d, want %d — a refused consumption enqueues nothing", got, entriesBefore)
	}
}
