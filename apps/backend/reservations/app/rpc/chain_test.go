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

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app/rpc"
	servicev1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

var correlationShape = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func reserve(t *testing.T, h *harness, ctx context.Context) {
	t.Helper()
	var resp servicev1.ReserveResponse
	if err := h.invoke(ctx, "Reserve", &servicev1.ReserveRequest{OrderId: "o-1", ItemCount: 1}, &resp); err != nil {
		t.Fatalf("Reserve() = %v, want nil", err)
	}
}

func TestACallWithoutDeadlineIsRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.ReserveResponse
	err := h.invoke(context.Background(), "Reserve", &servicev1.ReserveRequest{OrderId: "o-1", ItemCount: 1}, &resp)

	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Reserve() without deadline = %v, want InvalidArgument (GRP-04)", err)
	}
	if got := h.store.WithinCalls(); got != 0 {
		t.Fatalf("WithinCalls() = %d, want 0 — the use case must not run", got)
	}
}

func TestMetadataBecomesTheMessageContextOfTheOutbox(t *testing.T) {
	h := newHarness(t, unlimited)
	ctx := metadata.AppendToOutgoingContext(withTenant(t),
		kernel.CorrelationKey, "corr-1", kernel.CausationKey, "bff-req-1", "traceparent", traceparent)

	reserve(t, h, ctx)

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

func TestAMalformedCorrelationIsReplaced(t *testing.T) {
	h := newHarness(t, unlimited)
	ctx := metadata.AppendToOutgoingContext(withTenant(t), kernel.CorrelationKey, "not valid!")

	reserve(t, h, ctx)

	got := h.store.Entries()[0].Context.CorrelationID
	if got == "not valid!" || !correlationShape.MatchString(got) {
		t.Fatalf("CorrelationID = %q, want a minted correlation", got)
	}
}

func TestTheServerSpanContinuesThePropagatedTrace(t *testing.T) {
	h := newHarness(t, unlimited)
	ctx := metadata.AppendToOutgoingContext(withTenant(t), "traceparent", traceparent)

	reserve(t, h, ctx)

	want := "dmpf.grpc.server " + rpc.FullMethod("Reserve")
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

	var first, second servicev1.FindReservationResponse
	firstErr := h.invoke(withTenant(t), "FindReservation", &servicev1.FindReservationRequest{OrderId: "o-1"}, &first)
	secondErr := h.invoke(withTenant(t), "FindReservation", &servicev1.FindReservationRequest{OrderId: "o-1"}, &second)

	if status.Code(firstErr) != codes.NotFound {
		t.Fatalf("first FindReservation() = %v, want NotFound (admitted)", firstErr)
	}
	if status.Code(secondErr) != codes.ResourceExhausted {
		t.Fatalf("second FindReservation() = %v, want ResourceExhausted (RES-17)", secondErr)
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
	ctx := metadata.AppendToOutgoingContext(withTenant(t), kernel.IdempotencyKey, "k-42")

	reserve(t, h, ctx)

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
	other := metadata.AppendToOutgoingContext(withDeadline(t), kernel.TenantKey, "globex")

	var a, b, c servicev1.FindReservationResponse
	first := h.invoke(withTenant(t), "FindReservation", &servicev1.FindReservationRequest{OrderId: "o-1"}, &a)
	second := h.invoke(withTenant(t), "FindReservation", &servicev1.FindReservationRequest{OrderId: "o-1"}, &b)
	another := h.invoke(other, "FindReservation", &servicev1.FindReservationRequest{OrderId: "o-1"}, &c)

	if status.Code(first) != codes.NotFound || status.Code(second) != codes.ResourceExhausted {
		t.Fatalf("first = %v, second = %v; want the first admitted and the second refused", first, second)
	}
	if status.Code(another) != codes.NotFound {
		t.Fatalf("another tenant's FindReservation() = %v, want NotFound (admitted): the buckets are per tenant", another)
	}
}

// CTX-11, locale column downstream: the locale the edge resolved is preserved,
// and the service default answers only when none or a malformed one arrived.
func TestTheServerPreservesTheLocaleTheEdgeResolved(t *testing.T) {
	cases := map[string]struct{ sent, want string }{
		"declared":  {"pt-BR", "pt-BR"},
		"absent":    {"", kernel.DefaultLocale},
		"malformed": {"pt BR", kernel.DefaultLocale},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, unlimited)
			ctx := withTenant(t)
			if c.sent != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, kernel.LocaleKey, c.sent)
			}

			var resp servicev1.FindReservationResponse
			_ = h.invoke(ctx, "FindReservation", &servicev1.FindReservationRequest{OrderId: "o-1"}, &resp)

			if got := h.execution.execution.Locale(); got != c.want {
				t.Fatalf("Locale() = %q, want %q", got, c.want)
			}
		})
	}
}
