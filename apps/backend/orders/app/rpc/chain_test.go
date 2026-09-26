package rpc_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app/rpc"
	servicev1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

var correlationShape = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func addItem(t *testing.T, h *harness, ctx context.Context) {
	t.Helper()
	var resp servicev1.AddItemResponse
	if err := h.invoke(ctx, "AddItem", &servicev1.AddItemRequest{OrderId: "o-1", Sku: "A", Quantity: 1}, &resp); err != nil {
		t.Fatalf("AddItem() = %v, want nil", err)
	}
}

func TestACallWithoutDeadlineIsRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.AddItemResponse
	err := h.invoke(context.Background(), "AddItem", &servicev1.AddItemRequest{OrderId: "o-1", Sku: "A", Quantity: 1}, &resp)

	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("AddItem() without deadline = %v, want InvalidArgument (GRP-04)", err)
	}
	if got := h.store.WithinCalls(); got != 0 {
		t.Fatalf("WithinCalls() = %d, want 0 — the use case must not run", got)
	}
}

func TestMetadataBecomesTheMessageContextOfTheOutbox(t *testing.T) {
	h := newHarness(t, unlimited)
	ctx := metadata.AppendToOutgoingContext(withTenant(t),
		kernelgrpc.CorrelationKey, "corr-1", kernelgrpc.CausationKey, "bff-req-1", "traceparent", traceparent)

	addItem(t, h, ctx)

	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("Entries() = %d, want 1", len(entries))
	}
	mc := entries[0].Context
	if mc.CorrelationID != "corr-1" {
		t.Fatalf("CorrelationID = %q, want corr-1 (CTX-07)", mc.CorrelationID)
	}
	if mc.CausationID == "" || mc.CausationID == "bff-req-1" || mc.CausationID == string(entries[0].MessageID) {
		t.Fatalf("CausationID = %q, want the context's own request id (CTX-08)", mc.CausationID)
	}
	if !spanCarries(h, tracing.KeyRequestID, mc.CausationID) {
		t.Fatalf("no server span carries %s = %q, want the causation to be this request (CTX-08)", tracing.KeyRequestID, mc.CausationID)
	}
	parts := strings.Split(mc.Traceparent, "-")
	if len(parts) != 4 || parts[1] != traceID || parts[2] == parentSpan {
		t.Fatalf("Traceparent = %q, want trace %s with the server span as parent", mc.Traceparent, traceID)
	}
}

func TestTheServerRebuildsTheExecutionContextFromTheMetadata(t *testing.T) {
	h := newHarness(t, unlimited)
	ctx := metadata.AppendToOutgoingContext(withDeadline(t),
		kernelgrpc.CorrelationKey, "corr-1", kernelgrpc.CausationKey, "bff-req-1", kernelgrpc.TenantKey, "acme", "traceparent", traceparent)

	addItem(t, h, ctx)

	if !h.execution.present {
		t.Fatalf("ExecutionContextFrom() = _, false, want the server to rebuild the context (CTX-02)")
	}
	execution := h.execution.execution

	tenant, scoped := execution.Tenant()
	if !scoped || string(tenant) != "acme" {
		t.Fatalf("Tenant() = %q, %t, want acme, true (CTX-13)", tenant, scoped)
	}
	if execution.CorrelationID() != "corr-1" {
		t.Fatalf("CorrelationID() = %q, want corr-1 (CTX-07)", execution.CorrelationID())
	}
	causation, caused := execution.CausationID()
	if !caused || causation != "bff-req-1" {
		t.Fatalf("CausationID() = %q, %t, want bff-req-1, true: the edge request is the preceding step (CTX-08)", causation, caused)
	}
	if execution.RequestID() == "" || execution.RequestID() == causation {
		t.Fatalf("RequestID() = %q, want an identifier of this execution, distinct from the causation", execution.RequestID())
	}
	if execution.TraceContext() != traceID {
		t.Fatalf("TraceContext() = %q, want the propagated trace %s (CTX-09)", execution.TraceContext(), traceID)
	}
	if execution.Deadline() <= 0 {
		t.Fatalf("Deadline() = %d, want the instant the call is governed by (GRP-04)", execution.Deadline())
	}
	if execution.Locale() != kernelgrpc.DefaultLocale {
		t.Fatalf("Locale() = %q, want %q", execution.Locale(), kernelgrpc.DefaultLocale)
	}
}

func TestMetadataAssertingASubjectResolvesNone(t *testing.T) {
	h := newHarness(t, unlimited)
	ctx := metadata.AppendToOutgoingContext(withDeadline(t),
		kernelgrpc.TenantKey, "acme", "x-subject-id", "forged", "authorization", "Bearer forged")

	addItem(t, h, ctx)

	if !h.execution.present {
		t.Fatalf("ExecutionContextFrom() = _, false, want the server to rebuild the context (CTX-02)")
	}
	if subject, identified := h.execution.execution.Subject(); identified {
		t.Fatalf("Subject() = %q, true, want absent: channel trust establishes the caller, not the subject (IDN-02)", subject)
	}
	if permissions := h.execution.execution.Permissions(); permissions != nil {
		t.Fatalf("Permissions() = %v, want nil: they do not cross the fan-out (CTX-12)", permissions)
	}
}

func TestWhatTheCallerLeavesOutStaysAbsent(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.AddItemResponse
	err := h.invoke(withDeadline(t), "AddItem", &servicev1.AddItemRequest{OrderId: "o-1", Sku: "A", Quantity: 1}, &resp)

	if status.Code(err) != codes.Internal {
		t.Fatalf("AddItem() without a tenant = %v, want the call refused: absence is not permission (IDN-15)", err)
	}
	if !h.execution.present {
		t.Fatalf("ExecutionContextFrom() = _, false, want the server to rebuild the context (CTX-02)")
	}
	if tenant, scoped := h.execution.execution.Tenant(); scoped {
		t.Fatalf("Tenant() = %q, true, want absent: no filler value stands for an unresolved tenant (IDN-20)", tenant)
	}
	if causation, caused := h.execution.execution.CausationID(); caused {
		t.Fatalf("CausationID() = %q, true, want absent: no preceding step was declared (CTX-08)", causation)
	}
}

func TestAnEmptyTenantIsAbsenceAndNotADefect(t *testing.T) {
	h := newHarness(t, unlimited)
	ctx := metadata.AppendToOutgoingContext(withDeadline(t), kernelgrpc.TenantKey, "")

	var resp servicev1.AddItemResponse
	err := h.invoke(ctx, "AddItem", &servicev1.AddItemRequest{OrderId: "o-1", Sku: "A", Quantity: 1}, &resp)

	if status.Code(err) != codes.Internal {
		t.Fatalf("AddItem() with an empty tenant = %v, want the call refused: the empty value resolves no scope (IDN-15)", err)
	}
	if !h.execution.present {
		t.Fatalf("ExecutionContextFrom() = _, false, want the empty value to leave the context assemblable (CTX-01)")
	}
	if tenant, scoped := h.execution.execution.Tenant(); scoped {
		t.Fatalf("Tenant() = %q, true, want absent: the empty value never stands for a resolved tenant (IDN-20)", tenant)
	}
}

func TestAMalformedCorrelationIsReplaced(t *testing.T) {
	h := newHarness(t, unlimited)
	ctx := metadata.AppendToOutgoingContext(withTenant(t), kernelgrpc.CorrelationKey, "not valid!")

	addItem(t, h, ctx)

	got := h.store.Entries()[0].Context.CorrelationID
	if got == "not valid!" || !correlationShape.MatchString(got) {
		t.Fatalf("CorrelationID = %q, want a minted correlation", got)
	}
}

func TestTheServerSpanContinuesThePropagatedTrace(t *testing.T) {
	h := newHarness(t, unlimited)
	ctx := metadata.AppendToOutgoingContext(withTenant(t), "traceparent", traceparent)

	addItem(t, h, ctx)

	want := "dmpf.grpc.server " + rpc.FullMethod("AddItem")
	for _, span := range h.spans.GetSpans() {
		if span.Name != want {
			continue
		}
		if span.SpanKind != trace.SpanKindServer {
			t.Fatalf("kind = %v, want server", span.SpanKind)
		}
		if span.Parent.TraceID().String() != traceID || span.Parent.SpanID().String() != parentSpan {
			t.Fatalf("parent = %s/%s, want %s/%s (TRC-07)", span.Parent.TraceID(), span.Parent.SpanID(), traceID, parentSpan)
		}
		return
	}
	t.Fatalf("no span named %q among %d", want, len(h.spans.GetSpans()))
}

func TestAdmissionRefusesBeyondTheLimit(t *testing.T) {
	h := newHarness(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 1})

	var first, second servicev1.FindOrderResponse
	firstErr := h.invoke(withTenant(t), "FindOrder", &servicev1.FindOrderRequest{OrderId: "o-1"}, &first)
	secondErr := h.invoke(withTenant(t), "FindOrder", &servicev1.FindOrderRequest{OrderId: "o-1"}, &second)

	if status.Code(firstErr) != codes.NotFound {
		t.Fatalf("first FindOrder() = %v, want NotFound (admitted)", firstErr)
	}
	if status.Code(secondErr) != codes.ResourceExhausted {
		t.Fatalf("second FindOrder() = %v, want ResourceExhausted (RES-17)", secondErr)
	}
}

func TestTheHealthProbePassesThroughTheChain(t *testing.T) {
	h := newHarness(t, unlimited)
	client := healthpb.NewHealthClient(h.conn)

	resp, err := client.Check(withDeadline(t), &healthpb.HealthCheckRequest{Service: rpc.ServiceName})
	if err != nil {
		t.Fatalf("Check() = %v, want the probe answered: the health service declares no admission limit and is not a use case", err)
	}
	if got := resp.GetStatus(); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("status = %v before the server is ready, want NOT_SERVING", got)
	}
	if _, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{Service: rpc.ServiceName}); err != nil {
		t.Fatalf("Check() without deadline = %v, want the probe answered: GRP-04 binds the use case, not the probe", err)
	}
}

func TestTheIdempotencyKeyReachesTheLog(t *testing.T) {
	h := newHarness(t, unlimited)
	ctx := metadata.AppendToOutgoingContext(withTenant(t), kernelgrpc.IdempotencyKey, "k-42")

	addItem(t, h, ctx)

	if !strings.Contains(h.logs.String(), `"k-42"`) {
		t.Fatalf("logs = %s, want the idempotency key the BFF propagated", h.logs.String())
	}
}

func spanCarries(h *harness, key, value string) bool {
	for _, span := range h.spans.GetSpans() {
		for _, attribute := range span.Attributes {
			if string(attribute.Key) == key && attribute.Value.AsString() == value {
				return true
			}
		}
	}
	return false
}

// RES-16: the bucket is per tenant, keyed by the tenant the edge propagated, so
// one tenant exhausting its own does not refuse another's call.
func TestOneTenantExhaustingItsBucketDoesNotRefuseAnother(t *testing.T) {
	h := newHarness(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 1})
	other := metadata.AppendToOutgoingContext(withDeadline(t), kernelgrpc.TenantKey, "globex")

	var a, b, c servicev1.FindOrderResponse
	first := h.invoke(withTenant(t), "FindOrder", &servicev1.FindOrderRequest{OrderId: "o-1"}, &a)
	second := h.invoke(withTenant(t), "FindOrder", &servicev1.FindOrderRequest{OrderId: "o-1"}, &b)
	another := h.invoke(other, "FindOrder", &servicev1.FindOrderRequest{OrderId: "o-1"}, &c)

	if status.Code(first) != codes.NotFound || status.Code(second) != codes.ResourceExhausted {
		t.Fatalf("first = %v, second = %v; want the first admitted and the second refused", first, second)
	}
	if status.Code(another) != codes.NotFound {
		t.Fatalf("another tenant's FindOrder() = %v, want NotFound (admitted): the buckets are per tenant", another)
	}
}

// CTX-11, locale column downstream: the locale the edge resolved is preserved,
// and the service default answers only when none or a malformed one arrived.
func TestTheServerPreservesTheLocaleTheEdgeResolved(t *testing.T) {
	cases := map[string]struct{ sent, want string }{
		"declared":  {"pt-BR", "pt-BR"},
		"absent":    {"", kernelgrpc.DefaultLocale},
		"malformed": {"pt BR", kernelgrpc.DefaultLocale},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, unlimited)
			ctx := withTenant(t)
			if c.sent != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, kernelgrpc.LocaleKey, c.sent)
			}

			addItem(t, h, ctx)

			if got := h.execution.execution.Locale(); got != c.want {
				t.Fatalf("Locale() = %q, want %q", got, c.want)
			}
		})
	}
}
