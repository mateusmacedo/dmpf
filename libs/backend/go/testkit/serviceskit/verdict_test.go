package serviceskit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	ordersapp "github.com/mateusmacedo/dmpf/libs/backend/go/application/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const at = ports.Instant(1_755_432_000_000_000_000)

// uowOf adapts the recording unit of work, typed over `any` because the kit
// cannot know a service's resource type, to the type the service declares.
type uowOf[R any] struct{ inner ports.UnitOfWork[any] }

func (u uowOf[R]) Within(ctx context.Context, fn func(context.Context, R) error) error {
	return u.inner.Within(ctx, func(ctx context.Context, res any) error { return fn(ctx, res.(R)) })
}

func ordersService(f *serviceskit.Fakes) ordersapp.Service {
	uow := f.UnitOfWork(func(tx serviceskit.Tx) any {
		return ordersapp.Resources{
			Orders: serviceskit.Repository(tx, tx.Memory().Orders(), func(id orders.OrderID) string { return string(id) }),
			Outbox: tx.Outbox(),
		}
	})
	return ordersapp.Service{
		UoW:       uowOf[ordersapp.Resources]{inner: uow},
		Reader:    f.Store.Reader(),
		Clock:     clock.New(at),
		IDs:       &ids.Sequence{Prefix: "m-"},
		Authorize: application.AllowAll[ordersapp.Command](),
		ItemLimit: 2,
	}
}

func TestAcceptedCommitsStateAndOutboxTogether(t *testing.T) {
	f := serviceskit.NewFakes()
	svc := ordersService(f)
	outcome, err := svc.AddItem(context.Background(), ordersapp.AddItem{Order: "o-1", SKU: "A", Quantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, refused := outcome.Rejection(); refused {
		t.Fatal("AddItem refused on an empty order")
	}
	if got := f.Ledger.String(); got != "[begin write(o-1) enqueue(m-000001) commit]" {
		t.Fatalf("ledger = %s", got)
	}
	v := serviceskit.Decide(f, serviceskit.Expect{Accepted: true})
	tb.Require(t, v)
	evidence.RecordVerdict(t, "services", "accepted", v)
}

func TestRejectedLeavesNothingBehind(t *testing.T) {
	f := serviceskit.NewFakes()
	svc := ordersService(f)
	ctx := context.Background()
	for _, sku := range []orders.SKU{"A", "B"} {
		if _, err := svc.AddItem(ctx, ordersapp.AddItem{Order: "o-1", SKU: sku, Quantity: 1}); err != nil {
			t.Fatal(err)
		}
	}
	f.Ledger.Reset()
	before := len(f.Store.Entries())

	outcome, err := svc.AddItem(ctx, ordersapp.AddItem{Order: "o-1", SKU: "C", Quantity: 1})
	if err != nil {
		t.Fatal(err)
	}
	rej, refused := outcome.Rejection()
	if !refused || rej.Code() != orders.CodeItemLimitExceeded {
		t.Fatalf("outcome = %+v, want item-limit-exceeded", outcome)
	}
	if got := f.Ledger.String(); got != "[begin commit]" {
		t.Fatalf("ledger = %s, want an effect-free commit (UOW-06 rationale)", got)
	}
	if len(f.Store.Entries()) != before {
		t.Fatal("the outbox grew under a refusal")
	}
	// A fresh store shows the whole of UOW-06: after two accepted commands on a
	// limit of two, the third is refused and the store holds exactly the two
	// entries the accepted ones enqueued — Decide judges the refused command by
	// its ledger, so the fixture's entries are discounted.
	fresh := serviceskit.NewFakes()
	svc = ordersService(fresh)
	for _, sku := range []orders.SKU{"A", "B"} {
		if _, err := svc.AddItem(ctx, ordersapp.AddItem{Order: "o-1", SKU: sku, Quantity: 1}); err != nil {
			t.Fatal(err)
		}
	}
	fresh.Ledger.Reset()
	fresh.Baseline()
	if _, err := svc.AddItem(ctx, ordersapp.AddItem{Order: "o-1", SKU: "C", Quantity: 1}); err != nil {
		t.Fatal(err)
	}
	v := serviceskit.Decide(fresh, serviceskit.Expect{Accepted: false})
	tb.Require(t, v)
	evidence.RecordVerdict(t, "services", "rejected", v)
}

// The negative vector: a service that enqueues outside the transaction.
func TestEnqueueBeforeBeginIsNamedWithItsPosition(t *testing.T) {
	f := serviceskit.NewFakes()
	var stray ports.Outbox
	uow := f.UnitOfWork(func(tx serviceskit.Tx) any { stray = tx.Outbox(); return tx })
	ctx := context.Background()
	// First transaction only captures a transactional port and lets it escape.
	if err := uow.Within(ctx, func(context.Context, any) error { return nil }); err != nil {
		t.Fatal(err)
	}
	f.Ledger.Reset()
	if err := stray.Enqueue(ctx, ports.OutboxEntry{MessageID: "m-stray", OccurredAt: 1, Intent: ports.PublishIntent{Destination: "x", PartitionKey: "k"}, AggregateType: "t", AggregateID: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := uow.Within(ctx, func(context.Context, any) error { return nil }); err != nil {
		t.Fatal(err)
	}
	v := serviceskit.Decide(f, serviceskit.Expect{Accepted: true})
	if v.OK() {
		t.Fatal("an enqueue outside the transaction passed")
	}
	d := v.Diagnostics[0]
	if d.Rule != "UOW-07" || d.Seq != 0 || !strings.Contains(d.Detail, "enqueue(m-stray) outside any transaction") {
		t.Fatalf("diagnostic = %+v, want UOW-07 at position 0 naming the enqueue", d)
	}
}

func TestPublishingInsideTheSequenceIsReproved(t *testing.T) {
	f := serviceskit.NewFakes()
	pub := f.Publisher()
	uow := f.UnitOfWork(func(tx serviceskit.Tx) any { return tx })
	err := uow.Within(context.Background(), func(ctx context.Context, res any) error {
		return pub.Publish(ctx, "orders.events", []byte("x"))
	})
	if err != nil {
		t.Fatal(err)
	}
	v := serviceskit.Decide(f, serviceskit.Expect{Accepted: true})
	var named bool
	for _, d := range v.Diagnostics {
		if d.Rule == "UOW-08" && d.Seq == 1 {
			named = true
		}
	}
	if !named {
		t.Fatalf("UOW-08 at position 1 not named: %v", v.Failures())
	}
}

// A port that escaped an earlier transaction and is used inside a later one
// lands in a discarded copy (memory/tx.go): the ledger names it by its
// transaction number, not by its position between begin and commit.
func TestAPortOfAnotherTransactionIsNamed(t *testing.T) {
	f := serviceskit.NewFakes()
	var stray ports.Outbox
	// bind runs on every Within: capture the port of the first transaction only.
	uow := f.UnitOfWork(func(tx serviceskit.Tx) any {
		if stray == nil {
			stray = tx.Outbox()
		}
		return tx
	})
	ctx := context.Background()
	if err := uow.Within(ctx, func(context.Context, any) error { return nil }); err != nil {
		t.Fatal(err)
	}
	f.Ledger.Reset()
	f.Baseline()
	err := uow.Within(ctx, func(ctx context.Context, res any) error {
		return stray.Enqueue(ctx, ports.OutboxEntry{MessageID: "m-stray", OccurredAt: 1, Intent: ports.PublishIntent{Destination: "x", PartitionKey: "k"}, AggregateType: "t", AggregateID: "a"})
	})
	if err != nil {
		t.Fatal(err)
	}
	v := serviceskit.Decide(f, serviceskit.Expect{Accepted: true})
	var named bool
	for _, d := range v.Diagnostics {
		if d.Rule == "UOW-07" && strings.Contains(d.Detail, "port of transaction 1 while transaction 2 is open") {
			named = true
		}
	}
	if !named {
		t.Fatalf("the escaped port was not named: %v", v.Failures())
	}
}
