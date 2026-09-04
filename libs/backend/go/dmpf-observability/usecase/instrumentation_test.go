package usecase_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/audit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/usecase"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

const readOperation = "orders.FindOrder"

func harness(t *testing.T, subject usecase.SubjectFunc) (*usecase.Instrumentation, *tracetest.SpanRecorder, *audit.Recording) {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})

	recording := &audit.Recording{}
	instr := usecase.New(provider.Tracer("dmpf-observability"), recording, subject, readOperation)
	return instr, recorder, recording
}

func onlyEndedSpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want exactly 1", len(ended))
	}
	return ended[0]
}

func attributeOf(span sdktrace.ReadOnlySpan, key string) (string, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key && kv.Value.Type() == attribute.STRING {
			return kv.Value.AsString(), true
		}
	}
	return "", false
}

func TestBeginOperationOpensTheUseCaseSpanAsWriteTraffic(t *testing.T) {
	instr, recorder, _ := harness(t, nil)

	_, end := instr.BeginOperation(context.Background(), "orders.AddItem")
	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})

	span := onlyEndedSpan(t, recorder)
	if got := span.Name(); got != "dmpf.usecase.orders.AddItem" {
		t.Fatalf("span name = %q, want %q", got, "dmpf.usecase.orders.AddItem")
	}
	class, ok := attributeOf(span, "dmpf.traffic_class")
	if !ok || class != "write" {
		t.Fatalf("dmpf.traffic_class = %q (present=%v), want \"write\"", class, ok)
	}
}

func TestADeclaredReadOperationCarriesTheReadTrafficClass(t *testing.T) {
	instr, recorder, _ := harness(t, nil)

	_, end := instr.BeginOperation(context.Background(), readOperation)
	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})

	class, ok := attributeOf(onlyEndedSpan(t, recorder), "dmpf.traffic_class")
	if !ok || class != "read" {
		t.Fatalf("dmpf.traffic_class = %q (present=%v), want \"read\" for a declared read", class, ok)
	}
}

func TestTheTrafficClassIsSetAtStartSoTheSamplerSeesIt(t *testing.T) {
	instr, recorder, _ := harness(t, nil)

	_, end := instr.BeginOperation(context.Background(), "orders.AddItem")

	started := recorder.Started()
	if len(started) != 1 {
		t.Fatalf("started spans = %d, want 1", len(started))
	}
	found := false
	for _, kv := range started[0].Attributes() {
		if kv.Key == attribute.Key("dmpf.traffic_class") {
			found = true
		}
	}
	if !found {
		t.Fatal("dmpf.traffic_class absent at span start: the sampler only sees attributes given at creation")
	}
	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})
}

func TestEndOperationRecordsTheOutcomeCategory(t *testing.T) {
	for _, outcome := range []dmpfports.OutcomeCategory{
		dmpfports.OutcomeAccepted, dmpfports.OutcomeRejected, dmpfports.OutcomeDenied,
	} {
		t.Run(string(outcome), func(t *testing.T) {
			instr, recorder, _ := harness(t, nil)

			_, end := instr.BeginOperation(context.Background(), "orders.AddItem")
			end(dmpfports.Result{Outcome: outcome})

			span := onlyEndedSpan(t, recorder)
			got, ok := attributeOf(span, "dmpf.outcome_category")
			if !ok || got != string(outcome) {
				t.Fatalf("dmpf.outcome_category = %q (present=%v), want %q", got, ok, outcome)
			}
			if span.Status().Code == codes.Error {
				t.Fatalf("status = Error for outcome %q, want unset — only a technical failure is an error", outcome)
			}
		})
	}
}

func TestEndOperationOnFailedSetsTheErrorStatusWithoutAMessage(t *testing.T) {
	instr, recorder, _ := harness(t, nil)

	_, end := instr.BeginOperation(context.Background(), "orders.AddItem")
	end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: errors.New("storage unavailable")})

	span := onlyEndedSpan(t, recorder)
	if span.Status().Code != codes.Error {
		t.Fatalf("status = %v, want Error", span.Status().Code)
	}
	if span.Status().Description != "" {
		t.Fatalf("status description = %q, want empty: the error message never reaches the span (TRC-12)", span.Status().Description)
	}
}

func TestAuditForwardsToTheSinkWithTheResolvedSubject(t *testing.T) {
	instr, _, recording := harness(t, func(context.Context) string { return "svc-a" })

	instr.Audit(context.Background(), dmpfports.AuditEvent{
		Object:  "P-100",
		Action:  "orders.AddItem",
		Outcome: dmpfports.OutcomeAccepted,
		At:      dmpfports.Instant(1_755_432_000_000_000_000),
	})

	events := recording.Events()
	if len(events) != 1 {
		t.Fatalf("Events() has %d events, want 1", len(events))
	}
	want := audit.Event{
		Subject: "svc-a",
		Object:  "P-100",
		Action:  "orders.AddItem",
		Outcome: "accepted",
		At:      dmpfports.Instant(1_755_432_000_000_000_000),
	}
	if events[0] != want {
		t.Fatalf("Event = %+v, want %+v", events[0], want)
	}
}

func TestAuditWithoutASubjectFuncRecordsTheSubjectAsAbsent(t *testing.T) {
	instr, _, recording := harness(t, nil)

	instr.Audit(context.Background(), dmpfports.AuditEvent{Object: "P-100", Action: "orders.AddItem"})

	if got := recording.Events()[0].Subject; got != "" {
		t.Fatalf("Subject = %q, want the empty string: an absent identity is recorded as absent, never invented", got)
	}
}

func TestInstrumentationSatisfiesThePort(t *testing.T) {
	var _ dmpfports.Instrumentation = usecase.New(nil, nil, nil)
}
