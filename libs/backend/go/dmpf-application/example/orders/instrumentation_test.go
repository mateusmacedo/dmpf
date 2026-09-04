package ordersapp_test

import (
	"context"
	"errors"
	"testing"

	ordersapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/orders"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

type instrumentationRecorder struct {
	rec     *recorder
	begins  []string
	results []dmpfports.Result
	audits  []dmpfports.AuditEvent
}

func (r *instrumentationRecorder) BeginOperation(ctx context.Context, operation string) (context.Context, dmpfports.EndOperation) {
	r.rec.record("begin")
	r.begins = append(r.begins, operation)
	return ctx, func(result dmpfports.Result) {
		r.rec.record("end")
		r.results = append(r.results, result)
	}
}

func (r *instrumentationRecorder) Audit(_ context.Context, event dmpfports.AuditEvent) {
	r.rec.record("audit")
	r.audits = append(r.audits, event)
}

func newInstrumentedHarness(t *testing.T, options ...option) (*harness, *instrumentationRecorder) {
	t.Helper()

	h := newHarness(t, options...)
	instr := &instrumentationRecorder{rec: h.rec}
	h.service.Instrumentation = instr
	return h, instr
}

func (r *instrumentationRecorder) onlyResult(t *testing.T) dmpfports.Result {
	t.Helper()
	if len(r.results) != 1 {
		t.Fatalf("EndOperation called %d times, want exactly 1", len(r.results))
	}
	return r.results[0]
}

func TestAddItemAcceptedReportsAcceptedAndAudits(t *testing.T) {
	h, instr := newInstrumentedHarness(t)

	_, err := h.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if err != nil {
		t.Fatalf("AddItem() error = %v, want nil", err)
	}
	if got := instr.begins; len(got) != 1 || got[0] != ordersapp.OperationAddItem {
		t.Fatalf("BeginOperation operations = %v, want [%q]", got, ordersapp.OperationAddItem)
	}
	if got := instr.onlyResult(t); got.Outcome != dmpfports.OutcomeAccepted || got.Err != nil {
		t.Fatalf("Result = %+v, want {accepted, nil}", got)
	}
	if len(instr.audits) != 1 {
		t.Fatalf("Audit called %d times, want 1 — the fact was committed (LOG-14)", len(instr.audits))
	}
	want := dmpfports.AuditEvent{
		Object:  string(orderID),
		Action:  ordersapp.OperationAddItem,
		Outcome: dmpfports.OutcomeAccepted,
		At:      occurred,
	}
	if instr.audits[0] != want {
		t.Fatalf("AuditEvent = %+v, want %+v", instr.audits[0], want)
	}
}

func TestAddItemRejectedReportsRejectedAndAudits(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	h.seed(t, openSnapshot(itemLimit), 0)

	out, err := h.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if err != nil {
		t.Fatalf("AddItem() error = %v, want nil — a refusal is not a technical failure (DEC-04)", err)
	}
	if _, refused := out.Rejection(); !refused {
		t.Fatal("Rejection() reported no refusal, want orders/item-limit-exceeded")
	}
	if got := instr.onlyResult(t); got.Outcome != dmpfports.OutcomeRejected || got.Err != nil {
		t.Fatalf("Result = %+v, want {rejected, nil} — a refusal carries no technical error", got)
	}
	if len(instr.audits) != 1 || instr.audits[0].Outcome != dmpfports.OutcomeRejected {
		t.Fatalf("audits = %+v, want exactly one rejected event — the transaction committed", instr.audits)
	}
}

func TestAddItemDeniedReportsDeniedWithoutAuditOrTransaction(t *testing.T) {
	denied := func(context.Context, ordersapp.Command) error {
		return errors.Join(errors.New("policy engine refused"), dmpfports.ErrDenied)
	}
	h, instr := newInstrumentedHarness(t, withAuthorize(denied))

	_, err := h.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if !errors.Is(err, dmpfports.ErrDenied) {
		t.Fatalf("AddItem() error = %v, want a denial", err)
	}
	got := instr.onlyResult(t)
	if got.Outcome != dmpfports.OutcomeDenied {
		t.Fatalf("Result.Outcome = %q, want denied", got.Outcome)
	}
	if got.Err != nil {
		t.Fatalf("Result.Err = %v, want nil — a denial is not a technical failure", got.Err)
	}
	if len(instr.audits) != 0 {
		t.Fatalf("audits = %+v, want none — nothing was accessed", instr.audits)
	}
	if n := h.serviceWithinCalls(); n != 0 {
		t.Fatalf("transactions opened = %d, want 0 — the sequence stops before step 2", n)
	}
}

func TestAddItemAuthorizerFailureReportsFailedNotDenied(t *testing.T) {
	broken := errors.New("timeout dialing the policy engine")
	h, instr := newInstrumentedHarness(t, withAuthorize(func(context.Context, ordersapp.Command) error {
		return broken
	}))

	_, err := h.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if !errors.Is(err, broken) {
		t.Fatalf("AddItem() error = %v, want the authorizer error", err)
	}
	got := instr.onlyResult(t)
	if got.Outcome != dmpfports.OutcomeFailed {
		t.Fatalf("Result.Outcome = %q, want failed — a denial is never inferred from an unrelated error", got.Outcome)
	}
	if !errors.Is(got.Err, broken) {
		t.Fatalf("Result.Err = %v, want the raw error for the provider to classify", got.Err)
	}
	if len(instr.audits) != 0 {
		t.Fatalf("audits = %+v, want none", instr.audits)
	}
}

func TestAddItemTechnicalFailureReportsFailedWithTheError(t *testing.T) {
	broken := errors.New("storage unavailable")
	h, instr := newInstrumentedHarness(t, withSaveError(broken))

	_, err := h.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if !errors.Is(err, broken) {
		t.Fatalf("AddItem() error = %v, want the storage error", err)
	}
	got := instr.onlyResult(t)
	if got.Outcome != dmpfports.OutcomeFailed {
		t.Fatalf("Result.Outcome = %q, want failed", got.Outcome)
	}
	if !errors.Is(got.Err, broken) {
		t.Fatalf("Result.Err = %v, want the raw error", got.Err)
	}
	if len(instr.audits) != 0 {
		t.Fatalf("audits = %+v, want none — nothing was committed", instr.audits)
	}
}

func TestPlaceOrderAcceptedReportsAcceptedAndAudits(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	h.seed(t, openSnapshot(1), 0)

	_, err := h.service.PlaceOrder(context.Background(), ordersapp.PlaceOrder{Order: orderID})

	if err != nil {
		t.Fatalf("PlaceOrder() error = %v, want nil", err)
	}
	if got := instr.begins; len(got) != 1 || got[0] != ordersapp.OperationPlaceOrder {
		t.Fatalf("BeginOperation operations = %v, want [%q]", got, ordersapp.OperationPlaceOrder)
	}
	if got := instr.onlyResult(t); got.Outcome != dmpfports.OutcomeAccepted {
		t.Fatalf("Result = %+v, want accepted", got)
	}
	want := dmpfports.AuditEvent{
		Object:  string(orderID),
		Action:  ordersapp.OperationPlaceOrder,
		Outcome: dmpfports.OutcomeAccepted,
		At:      occurred,
	}
	if len(instr.audits) != 1 || instr.audits[0] != want {
		t.Fatalf("audits = %+v, want exactly %+v", instr.audits, want)
	}
}

func TestFindOrderReportsAcceptedWithoutAudit(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	h.seed(t, openSnapshot(2), 0)

	if _, err := h.service.FindOrder(context.Background(), orderID); err != nil {
		t.Fatalf("FindOrder() error = %v, want nil", err)
	}

	if got := instr.begins; len(got) != 1 || got[0] != ordersapp.OperationFindOrder {
		t.Fatalf("BeginOperation operations = %v, want [%q]", got, ordersapp.OperationFindOrder)
	}
	if got := instr.onlyResult(t); got.Outcome != dmpfports.OutcomeAccepted {
		t.Fatalf("Result = %+v, want accepted", got)
	}
	if len(instr.audits) != 0 {
		t.Fatalf("audits = %+v, want none — a query accesses nothing auditable (LOG-14)", instr.audits)
	}
}

func TestFindOrderOnAnAbsentAggregateReportsFailed(t *testing.T) {
	h, instr := newInstrumentedHarness(t)

	if _, err := h.service.FindOrder(context.Background(), orderID); err == nil {
		t.Fatal("FindOrder() error = nil, want ErrNotFound for an absent aggregate")
	}

	if got := instr.onlyResult(t); got.Outcome != dmpfports.OutcomeFailed || got.Err == nil {
		t.Fatalf("Result = %+v, want failed carrying the error", got)
	}
}

func TestBeginPrecedesAuthorizationAndEndClosesTheSequence(t *testing.T) {
	h, _ := newInstrumentedHarness(t)

	if _, err := h.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1}); err != nil {
		t.Fatalf("AddItem() error = %v, want nil", err)
	}

	observed := h.rec.observed
	if len(observed) < 4 || observed[0] != "begin" {
		t.Fatalf("observed = %v, want begin first — the span opens before step 1 (TRC-16)", observed)
	}
	if observed[1] != "authorize" {
		t.Fatalf("observed = %v, want authorize right after begin", observed)
	}
	if got := observed[len(observed)-2:]; got[0] != "end" || got[1] != "audit" {
		t.Fatalf("observed = %v, want the sequence to close with end then audit", observed)
	}
}

func TestServiceWithoutInstrumentationRunsInertly(t *testing.T) {
	h := newHarness(t)

	out, err := h.service.AddItem(context.Background(), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1})

	if err != nil {
		t.Fatalf("AddItem() error = %v, want nil — a nil hook falls back to NoInstrumentation", err)
	}
	if got, want := out.Response(), (orders.ItemAccepted{Order: orderID, Items: 1}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
}
