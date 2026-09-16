package usecase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application/example/memory"
	ordersapp "github.com/mateusmacedo/dmpf/libs/backend/go/application/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	orderID   = orders.OrderID("P-100")
	occurred  = ports.Instant(1_755_432_000_000_000_000)
	itemLimit = 3
)

// wiring is the composition root of this suite: it is what binds the pure hook
// of the application block to the provider realization, which neither block may
// do for itself.
type wiring struct {
	service ordersapp.Service
	store   *memory.Store
	trail   *audit.Recording

	runtime  *otelboot.Runtime
	exporter *tracetest.InMemoryExporter
	reader   *sdkmetric.ManualReader
	log      *bytes.Buffer
}

func wire(t *testing.T, authorize application.AuthorizeFunc[ordersapp.Command]) *wiring {
	t.Helper()

	fixture := boot(t, options{
		subject:    func(context.Context) string { return "svc-a" },
		classifier: func(error) string { return "storage" },
	})
	trail := fixture.recording
	store := memory.New()
	bind := func(tx *memory.Tx) ordersapp.Resources {
		return ordersapp.Resources{Orders: tx.Orders(), Outbox: tx.Outbox()}
	}

	return &wiring{
		service: ordersapp.Service{
			UoW:             memory.NewUnitOfWork(store, bind),
			Reader:          store.Reader(),
			Clock:           memory.FixedClock{At: occurred},
			IDs:             &memory.SequenceIDs{Prefix: "m-"},
			Authorize:       authorize,
			ItemLimit:       itemLimit,
			Instrumentation: fixture.instrumentation,
		},
		store:    store,
		trail:    trail,
		runtime:  fixture.runtime,
		exporter: fixture.exporter,
		reader:   fixture.reader,
		log:      fixture.log,
	}
}

func (w *wiring) seed(t *testing.T, snapshot orders.Snapshot) {
	t.Helper()
	err := memory.NewUnitOfWork(w.store, func(tx *memory.Tx) *memory.Tx { return tx }).
		Within(context.Background(), func(ctx context.Context, tx *memory.Tx) error {
			return tx.Orders().Save(ctx, orderID, snapshot, 0)
		})
	if err != nil {
		t.Fatalf("seed = %v, want nil", err)
	}
}

func openSnapshot(items int) orders.Snapshot {
	built := make([]orders.Item, 0, items)
	for i := range items {
		built = append(built, orders.Item{SKU: orders.SKU(string(rune('A' + i))), Quantity: 1})
	}
	return orders.Snapshot{ID: orderID, Status: orders.Open, ItemLimit: itemLimit, Items: built}
}

func (w *wiring) endedSpan(t *testing.T) tracetest.SpanStub {
	t.Helper()

	if err := w.runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	ended := w.exporter.GetSpans()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1 — the span is born in the application service", len(ended))
	}
	return ended[0]
}

func allowAll() application.AuthorizeFunc[ordersapp.Command] {
	return application.AllowAll[ordersapp.Command]()
}

// budgeted is the context the platform hands a use case: the retry budget of
// RES-24 travels in it, and nothing else does.
func budgeted(t *testing.T) context.Context {
	t.Helper()

	ctx := retry.WithBudget(context.Background())
	if _, armed := retry.BudgetFrom(ctx); !armed {
		t.Fatal("BudgetFrom() found no budget in the context the test just built")
	}
	return ctx
}

// requestSeries indexes dmpf_service_requests_total by operation and outcome.
func (w *wiring) requestSeries(t *testing.T) map[string]int64 {
	t.Helper()

	byKey := make(map[string]int64)
	for _, point := range collect(t, w.reader)[metrics.RequestsTotal] {
		labels := labelsOf(point)
		byKey[labels[metrics.KeyOperation]+"/"+labels[metrics.KeyOutcomeCategory]] = point.Value
	}
	return byKey
}

func (w *wiring) records(t *testing.T) []map[string]any {
	t.Helper()

	text := strings.TrimSpace(w.log.String())
	if text == "" {
		return nil
	}

	parsed := make([]map[string]any, 0, 4)
	for line := range strings.SplitSeq(text, "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("a log record is not JSON: %v (line %q)", err, line)
		}
		parsed = append(parsed, record)
	}
	return parsed
}

func TestAcceptedProducesOneSpanOneAuditAndOneTransaction(t *testing.T) {
	w := wire(t, allowAll())

	out, err := w.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if err != nil {
		t.Fatalf("AddItem() error = %v, want nil", err)
	}
	if _, refused := out.Rejection(); refused {
		t.Fatal("Rejection() reported a refusal, want acceptance")
	}

	span := w.endedSpan(t)
	if got := span.Name; got != "dmpf.usecase.orders.AddItem" {
		t.Fatalf("span name = %q, want %q", got, "dmpf.usecase.orders.AddItem")
	}
	if got, ok := attributeOf(span, "dmpf.outcome_category"); !ok || got != "accepted" {
		t.Fatalf("dmpf.outcome_category = %q (present=%v), want \"accepted\"", got, ok)
	}
	if got, ok := attributeOf(span, "dmpf.traffic_class"); !ok || got != "write" {
		t.Fatalf("dmpf.traffic_class = %q (present=%v), want \"write\"", got, ok)
	}

	events := w.trail.Events()
	want := audit.Event{
		Subject: "svc-a",
		Object:  string(orderID),
		Action:  ordersapp.OperationAddItem,
		Outcome: "accepted",
		At:      occurred,
	}
	if len(events) != 1 || events[0] != want {
		t.Fatalf("audit trail = %+v, want exactly [%+v]", events, want)
	}

	if got := w.store.WithinCalls(); got != 1 {
		t.Fatalf("WithinCalls() = %d, want 1 — Within never repeats the callback (UOW-09)", got)
	}
}

func TestRejectedProducesOneAuditAndCommitsWithoutWriting(t *testing.T) {
	w := wire(t, allowAll())
	w.seed(t, openSnapshot(itemLimit))
	before := w.store.WithinCalls()

	out, err := w.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if err != nil {
		t.Fatalf("AddItem() error = %v, want nil — a refusal is not a technical failure (DEC-04)", err)
	}
	if _, refused := out.Rejection(); !refused {
		t.Fatal("Rejection() reported no refusal, want orders/item-limit-exceeded")
	}
	if got, ok := attributeOf(w.endedSpan(t), "dmpf.outcome_category"); !ok || got != "rejected" {
		t.Fatalf("dmpf.outcome_category = %q (present=%v), want \"rejected\"", got, ok)
	}
	if events := w.trail.Events(); len(events) != 1 || events[0].Outcome != "rejected" {
		t.Fatalf("audit trail = %+v, want exactly one rejected record — the transaction committed", events)
	}
	if got := w.store.WithinCalls() - before; got != 1 {
		t.Fatalf("transactions opened = %d, want 1", got)
	}
}

func TestDeniedProducesNoAuditAndNoTransaction(t *testing.T) {
	w := wire(t, func(context.Context, ordersapp.Command) error {
		return errors.Join(errors.New("policy engine refused"), ports.ErrDenied)
	})

	_, err := w.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if !errors.Is(err, ports.ErrDenied) {
		t.Fatalf("AddItem() error = %v, want a denial", err)
	}
	if got, ok := attributeOf(w.endedSpan(t), "dmpf.outcome_category"); !ok || got != "denied" {
		t.Fatalf("dmpf.outcome_category = %q (present=%v), want \"denied\"", got, ok)
	}
	if events := w.trail.Events(); len(events) != 0 {
		t.Fatalf("audit trail = %+v, want empty — nothing was accessed", events)
	}
	if got := w.store.WithinCalls(); got != 0 {
		t.Fatalf("WithinCalls() = %d, want 0 — the sequence stops before step 2", got)
	}
}

func TestAnAuthorizerErrorThatIsNotDeniedIsReportedAsFailed(t *testing.T) {
	broken := errors.New("timeout dialing the policy engine")
	w := wire(t, func(context.Context, ordersapp.Command) error { return broken })

	_, err := w.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if !errors.Is(err, broken) {
		t.Fatalf("AddItem() error = %v, want the authorizer error", err)
	}
	span := w.endedSpan(t)
	if got, ok := attributeOf(span, "dmpf.outcome_category"); !ok || got != "failed" {
		t.Fatalf("dmpf.outcome_category = %q (present=%v), want \"failed\" — a denial is never inferred", got, ok)
	}
	if events := w.trail.Events(); len(events) != 0 {
		t.Fatalf("audit trail = %+v, want empty", events)
	}
}

func TestFindOrderIsReadTrafficAndLeavesNoAuditTrail(t *testing.T) {
	w := wire(t, allowAll())
	w.seed(t, openSnapshot(2))
	before := w.store.WithinCalls()

	if _, err := w.service.FindOrder(context.Background(), orderID); err != nil {
		t.Fatalf("FindOrder() error = %v, want nil", err)
	}

	span := w.endedSpan(t)
	if got := span.Name; got != "dmpf.usecase.orders.FindOrder" {
		t.Fatalf("span name = %q, want %q", got, "dmpf.usecase.orders.FindOrder")
	}
	if got, ok := attributeOf(span, "dmpf.traffic_class"); !ok || got != "read" {
		t.Fatalf("dmpf.traffic_class = %q (present=%v), want \"read\"", got, ok)
	}
	if events := w.trail.Events(); len(events) != 0 {
		t.Fatalf("audit trail = %+v, want empty — a query accesses nothing auditable (LOG-14)", events)
	}
	if got := w.store.WithinCalls() - before; got != 0 {
		t.Fatalf("transactions opened = %d, want 0 — a query never opens one (UOW-11)", got)
	}
}
