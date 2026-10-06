package observe_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	lognoop "go.opentelemetry.io/otel/log/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

var errBoom = errors.New("boom: secret detail that must never leave the process")

func op() resilience.Operation {
	return resilience.Operation{Dependency: "orders", Method: "Place", Kind: resilience.Remote, Deadline: time.Second, EstimatedDuration: 100 * time.Millisecond}
}

func failing(ctx context.Context, _ resilience.Operation, _ func(context.Context) error) error {
	return errBoom
}

func recorder(t *testing.T) (*tracetest.SpanRecorder, trace.Tracer) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return recorder, tp.Tracer("t")
}

func attributesOf(span sdktrace.ReadOnlySpan) map[string]attribute.Value {
	attrs := map[string]attribute.Value{}
	for _, kv := range span.Attributes() {
		attrs[string(kv.Key)] = kv.Value
	}
	return attrs
}

func eventsNamed(span sdktrace.ReadOnlySpan, name string) []sdktrace.Event {
	var found []sdktrace.Event
	for _, event := range span.Events() {
		if event.Name == name {
			found = append(found, event)
		}
	}
	return found
}

func eventAttributes(event sdktrace.Event) map[string]attribute.Value {
	attrs := map[string]attribute.Value{}
	for _, kv := range event.Attributes {
		attrs[string(kv.Key)] = kv.Value
	}
	return attrs
}

type categorized struct{ category string }

func (e categorized) Error() string         { return "failure: secret detail" }
func (e categorized) ErrorCategory() string { return e.category }
func (e categorized) ErrorCode() string     { return "TEST-01" }

func TestTracingOpensTheInternalResilienceSpanOverTheAttempts(t *testing.T) {
	spans, tracer := recorder(t)
	c := clock.NewFake(start)
	ctx, cancel := c.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	cfg := observe.Config{Clock: c, Tracer: tracer, MaxAttempts: 3, Category: func(error) string { return "boom" }}
	err := observe.Tracing(cfg)(failing)(ctx, op(), nil)
	if !errors.Is(err, errBoom) {
		t.Fatalf("decorated call = %v, want the original error", err)
	}

	ended := spans.Ended()
	if len(ended) != 1 || ended[0].Name() != "dmpf.resilience orders" || ended[0].SpanKind() != trace.SpanKindInternal {
		t.Fatalf("spans = %v, want one INTERNAL dmpf.resilience orders (RF-B5)", ended)
	}
	attrs := attributesOf(ended[0])
	if attrs["dmpf.dependency"].AsString() != "orders" || attrs["dmpf.retry.max_attempts"].AsInt64() != 3 || attrs["dmpf.deadline.remaining_ms"].AsInt64() != 800 {
		t.Errorf("attributes = %v, want dependency, max_attempts and remaining deadline (RF-B5)", attrs)
	}
	if attrs["error.type"].AsString() != "boom" || attrs["dmpf.outcome_category"].AsString() != "boom" {
		t.Errorf("categories = %v (TRC-12)", attrs)
	}
	for _, key := range []string{"dmpf.service", "dmpf.operation"} {
		if _, present := attrs[key]; present {
			t.Errorf("attribute %s is on the span, want it out (RF-B1)", key)
		}
	}
	if ended[0].Status().Description != "" {
		t.Errorf("status description %q leaks the message (TRC-12)", ended[0].Status().Description)
	}
	for key, value := range attrs {
		if strings.Contains(value.String(), "secret") {
			t.Errorf("attribute %s carries the message", key)
		}
	}
}

func TestAFailureWithoutACategoryFunctionIsTheTypeOther(t *testing.T) {
	spans, tracer := recorder(t)
	provider, logs := memoryLogs(slog.LevelInfo)
	cfg := observe.Config{Tracer: tracer, LoggerProvider: provider}

	_ = observe.Tracing(cfg)(observe.Logging(cfg)(failing))(context.Background(), op(), nil)

	other := semconv.ErrorTypeOther.Value.AsString()
	attrs := attributesOf(spans.Ended()[0])
	if attrs["error.type"].AsString() != other || attrs["dmpf.outcome_category"].AsString() != other {
		t.Errorf("span categories = %v, want error.type and dmpf.outcome_category %q (TRC-12)", attrs, other)
	}
	got := logs.maps()
	if len(got) != 1 || got[0]["error.type"] != other || got[0]["dmpf.outcome_category"] != other {
		t.Errorf("log = %v, want the failed attempt with error.type %q (RF-B1)", got, other)
	}
}

func TestTracingWithoutADeadlineOrACeilingOmitsBoth(t *testing.T) {
	spans, tracer := recorder(t)
	_ = observe.Tracing(observe.Config{Tracer: tracer})(resilience.Direct)(context.Background(), op(), func(context.Context) error { return nil })

	attrs := attributesOf(spans.Ended()[0])
	for _, key := range []string{"dmpf.deadline.remaining_ms", "dmpf.retry.max_attempts", "error.type"} {
		if _, present := attrs[key]; present {
			t.Errorf("attribute %s = %v on a call that declares none", key, attrs[key])
		}
	}
	if attrs["dmpf.outcome_category"].AsString() != observe.CategoryOK {
		t.Errorf("outcome = %v, want %q", attrs["dmpf.outcome_category"], observe.CategoryOK)
	}
}

func TestEveryRetryLeavesAnEventWithThePreviousCategory(t *testing.T) {
	spans, tracer := recorder(t)
	c := clock.NewFake(start)
	retrying, err := resilience.NewRetry(resilience.RetryConfig{
		Dependency:  "orders",
		Classifier:  func(error) retry.Retryability { return retry.Retryable },
		MaxAttempts: 3,
		Backoff:     resilience.BackoffPolicy{Base: time.Millisecond, Factor: 2, Cap: 10 * time.Millisecond},
		Rand:        func() float64 { return 0 },
	}, c, func(context.Context, time.Duration) error { return nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = retry.WithBudget(ctx, retry.WithTotal(10*time.Second))

	failures := []error{categorized{"unavailable"}, categorized{"timeout"}}
	attempts := 0
	idempotent := op()
	idempotent.Idempotent = true
	call := observe.Tracing(observe.Config{Clock: c, Tracer: tracer, MaxAttempts: 3})(retrying(resilience.Direct))
	err = call(ctx, idempotent, func(context.Context) error {
		attempts++
		if attempts <= len(failures) {
			return failures[attempts-1]
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("call = %v after %d attempts, want nil after 3", err, attempts)
	}

	ended := spans.Ended()
	if len(ended) != 1 {
		t.Fatalf("%d spans ended, want the one resilience span over the attempts", len(ended))
	}
	events := eventsNamed(ended[0], "dmpf.retry.attempt")
	if len(events) != 2 {
		t.Fatalf("events = %v, want one per retry", ended[0].Events())
	}
	for i, want := range []string{"unavailable", "timeout"} {
		attrs := eventAttributes(events[i])
		if attrs["dmpf.retry.attempt"].AsInt64() != int64(i+2) || attrs["dmpf.retry.previous_category"].AsString() != want {
			t.Errorf("event %d = %v, want attempt %d after %q", i, attrs, i+2, want)
		}
	}
	if got := attributesOf(ended[0])["dmpf.outcome_category"].AsString(); got != observe.CategoryOK {
		t.Errorf("outcome = %q, want the final one, %q", got, observe.CategoryOK)
	}
}

func TestUnderTheOwnedSendNoSpanOpensAndTheSendCarriesTheResilience(t *testing.T) {
	spans, tracer := recorder(t)
	c := clock.NewFake(start)
	deadline, cancel := c.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	ctx, send := tracer.Start(deadline, "send orders.topic", trace.WithSpanKind(trace.SpanKindClient))
	ctx = tracing.WithOwnedSpan(ctx, send)

	cfg := observe.Config{Clock: c, Tracer: tracer, MaxAttempts: 5, Category: func(error) string { return "boom" }}
	err := observe.Tracing(cfg)(func(ctx context.Context, op resilience.Operation, do func(context.Context) error) error {
		if trace.SpanFromContext(ctx).SpanContext().SpanID() != send.SpanContext().SpanID() {
			t.Error("the call below does not run under the owned send")
		}
		tracing.AttemptEvent(trace.SpanFromContext(ctx), 1, "unavailable")
		return errBoom
	})(ctx, op(), nil)
	if !errors.Is(err, errBoom) {
		t.Fatalf("decorated call = %v, want the original error", err)
	}
	if ended := spans.Ended(); len(ended) != 0 {
		t.Fatalf("spans ended before the owner = %v, want none: the send is ended only by its owner", ended)
	}

	send.End()
	ended := spans.Ended()
	if len(ended) != 1 || ended[0].Name() != "send orders.topic" {
		t.Fatalf("spans = %v, want only the owned send", ended)
	}
	attrs := attributesOf(ended[0])
	if attrs["dmpf.dependency"].AsString() != "orders" || attrs["dmpf.retry.max_attempts"].AsInt64() != 5 || attrs["dmpf.deadline.remaining_ms"].AsInt64() != 500 {
		t.Errorf("send attributes = %v, want the resilience attributes on it (RF-B5)", attrs)
	}
	if attrs["dmpf.outcome_category"].AsString() != "boom" || attrs["error.type"].AsString() != "boom" {
		t.Errorf("send attributes = %v, want the outcome on it", attrs)
	}
	if len(eventsNamed(ended[0], "dmpf.retry.attempt")) != 1 {
		t.Errorf("send events = %v, want the retry event on it", ended[0].Events())
	}
}

func TestARefusalLeavesItsEventOnTheResilienceSpan(t *testing.T) {
	cases := map[string]struct {
		next  func(t *testing.T) resilience.Call
		event string
		attrs map[string]string
	}{
		"breaker open": {
			next: func(t *testing.T) resilience.Call {
				breaker := resilience.NewBreaker("orders", resilience.BreakerPolicy{Threshold: 0.5, MinSamples: 1, Cooldown: time.Minute}, clock.NewFake(start), nil).
					LogsTo(lognoop.NewLoggerProvider())
				guarded := breaker.Decorate()(resilience.Direct)
				_ = guarded(context.Background(), op(), func(context.Context) error { return errBoom })
				if breaker.State() != resilience.BreakerOpen {
					t.Fatalf("breaker = %v, want open before the refused call", breaker.State())
				}
				return guarded
			},
			event: "dmpf.breaker.rejected",
			attrs: map[string]string{"dmpf.dependency": "orders"},
		},
		"bulkhead saturated": {
			next: func(*testing.T) resilience.Call {
				return func(ctx context.Context, op resilience.Operation, _ func(context.Context) error) error {
					tracing.BulkheadSaturated(trace.SpanFromContext(ctx), op.Dependency)
					return fmt.Errorf("%w: orders", resilience.ErrBulkheadSaturated)
				}
			},
			event: "dmpf.bulkhead.saturated",
		},
		"degraded": {
			next: func(t *testing.T) resilience.Call {
				degrade, err := resilience.NewDegradation(resilience.Degrade, "orders", nil)
				if err != nil {
					t.Fatal(err)
				}
				return degrade(failing)
			},
			event: "dmpf.degraded",
			attrs: map[string]string{"dmpf.error.code": "RES-37"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			next := tc.next(t)
			spans, tracer := recorder(t)
			_ = observe.Tracing(observe.Config{Tracer: tracer})(next)(context.Background(), op(), func(context.Context) error { return nil })

			ended := spans.Ended()
			if len(ended) != 1 {
				t.Fatalf("%d spans ended, want the resilience span", len(ended))
			}
			events := eventsNamed(ended[0], tc.event)
			if len(events) != 1 {
				t.Fatalf("events = %v, want exactly one %s", ended[0].Events(), tc.event)
			}
			got := eventAttributes(events[0])
			for key, want := range tc.attrs {
				if got[key].AsString() != want {
					t.Errorf("event %s = %v, want %s=%s", tc.event, got, key, want)
				}
			}
		})
	}
}

func TestASuccessLeavesNoRefusalEvent(t *testing.T) {
	spans, tracer := recorder(t)
	_ = observe.Tracing(observe.Config{Tracer: tracer})(resilience.Direct)(context.Background(), op(), func(context.Context) error { return nil })

	if events := spans.Ended()[0].Events(); len(events) != 0 {
		t.Fatalf("events = %v on a successful call", events)
	}
}

func TestLoggingWritesTheCategoryAndNotTheMessage(t *testing.T) {
	provider, logs := memoryLogs(slog.LevelInfo)
	cfg := observe.Config{LoggerProvider: provider, Category: func(error) string { return "boom" }}

	_ = observe.Logging(cfg)(failing)(context.Background(), op(), nil)

	got := logs.maps()
	if len(got) != 1 || got[0]["error.type"] != "boom" || got[0]["dmpf.dependency"] != "orders" || got[0]["dmpf.retry.attempt"] != int64(1) {
		t.Fatalf("log = %v, want the refused attempt with its category and dependency", got)
	}
	if got[0]["scope"] != observeScope {
		t.Fatalf("scope = %v, want the import path of the emitting package %q (RF-A1)", got[0]["scope"], observeScope)
	}
	if out := logs.text(); strings.Contains(out, "secret") {
		t.Fatalf("log = %q carries the error message (LOG-13)", out)
	}
	_ = observe.Logging(cfg)(resilience.Direct)(context.Background(), op(), func(context.Context) error { return nil })
	if after := logs.maps(); len(after) != 1 {
		t.Fatalf("a successful call logged %v", after[1:])
	}
}

func TestSlotsFillsExactlyTheThreeObservabilityPositions(t *testing.T) {
	slots := observe.Slots(observe.Config{})
	if slots.Tracing == nil || slots.Metrics == nil || slots.Logging == nil {
		t.Fatal("an observability position is empty (RES-23)")
	}
	if slots.Bulkhead != nil || slots.Breaker != nil || slots.RateLimit != nil || slots.Retry != nil || slots.Timeout != nil {
		t.Fatal("Slots filled a position that is the caller's")
	}
}

func TestTheMetricsPositionOnlyPassesTheCallThrough(t *testing.T) {
	slots := observe.Slots(observe.Config{Category: func(error) string { return "boom" }})
	if err := slots.Metrics(failing)(context.Background(), op(), nil); !errors.Is(err, errBoom) {
		t.Fatalf("call = %v, want the original error", err)
	}
	ran := false
	if err := slots.Metrics(resilience.Direct)(context.Background(), op(), func(context.Context) error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("call = %v, ran = %v", err, ran)
	}
}

func TestLoggingRecordsASuccessfulCallAtDebug(t *testing.T) {
	provider, logs := memoryLogs(slog.LevelDebug)

	_ = observe.Logging(observe.Config{LoggerProvider: provider})(resilience.Direct)(context.Background(), op(), func(context.Context) error { return nil })

	got := logs.maps()
	if len(got) != 1 {
		t.Fatalf("records = %v, want the one successful attempt", got)
	}
	want := map[string]any{"level": "DEBUG", "msg": "transport: call", "dmpf.dependency": "orders", "dmpf.retry.attempt": int64(1), "dmpf.outcome_category": "ok"}
	assertRecord(t, got[0], want, "error.type")
}

func assertRecord(t *testing.T, got, want map[string]any, absent ...string) {
	t.Helper()
	for key, value := range want {
		if got[key] != value {
			t.Errorf("record %v: %s = %v, want %v", got, key, got[key], value)
		}
	}
	for _, key := range append(absent, "duration_ms", "dependency", "operation", "error_category", "outcome_category") {
		if _, present := got[key]; present {
			t.Errorf("record %v carries %s (RF-A5)", got, key)
		}
	}
	for key, value := range got {
		if text, ok := value.(string); ok && strings.Contains(text, "secret") {
			t.Errorf("record %v: %s carries the error message (LOG-13)", got, key)
		}
	}
}

func categoryOf(err error) string {
	var c categorized
	if errors.As(err, &c) {
		return c.category
	}
	return "boom"
}

func TestLoggingWarnsOncePerFailedAttempt(t *testing.T) {
	cases := map[string]struct {
		failures []error
		final    error
	}{
		"recovered on the third attempt": {failures: []error{categorized{"unavailable"}, categorized{"timeout"}}},
		"exhausted":                      {failures: []error{categorized{"unavailable"}, categorized{"timeout"}, categorized{"unavailable"}}, final: categorized{"unavailable"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := clock.NewFake(start)
			retrying, err := resilience.NewRetry(resilience.RetryConfig{
				Dependency:  "orders",
				Classifier:  func(error) retry.Retryability { return retry.Retryable },
				MaxAttempts: 3,
				Backoff:     resilience.BackoffPolicy{Base: time.Millisecond, Factor: 2, Cap: 10 * time.Millisecond},
				Rand:        func() float64 { return 0 },
			}, c, func(context.Context, time.Duration) error { return nil }, nil)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			ctx = retry.WithBudget(ctx, retry.WithTotal(10*time.Second))
			idempotent := op()
			idempotent.Idempotent = true

			provider, logs := memoryLogs(slog.LevelDebug)
			call := observe.Logging(observe.Config{LoggerProvider: provider, Category: categoryOf})(retrying(resilience.Direct))
			attempts := 0
			err = call(ctx, idempotent, func(context.Context) error {
				attempts++
				if attempts <= len(tc.failures) {
					return tc.failures[attempts-1]
				}
				return nil
			})
			if !errors.Is(err, tc.final) {
				t.Fatalf("call = %v, want %v", err, tc.final)
			}

			got := logs.maps()
			if len(got) != attempts {
				t.Fatalf("records = %v, want one per attempt (%d)", got, attempts)
			}
			for i, failure := range tc.failures {
				category := categoryOf(failure)
				assertRecord(t, got[i], map[string]any{
					"level": "WARN", "msg": "transport: call failed", "dmpf.dependency": "orders",
					"dmpf.retry.attempt": int64(i + 1), "dmpf.outcome_category": category, "error.type": category,
				})
			}
			if tc.final == nil {
				assertRecord(t, got[attempts-1], map[string]any{
					"level": "DEBUG", "msg": "transport: call", "dmpf.retry.attempt": int64(attempts), "dmpf.outcome_category": "ok",
				}, "error.type")
			}
		})
	}
}

func TestLoggingWarnsTheAttemptRefusedBelowItOnce(t *testing.T) {
	refused := fmt.Errorf("%w: orders.Place has no time left", resilience.ErrDeadlineExceeded)
	cases := map[string]struct {
		next     resilience.Call
		attempts []string
	}{
		"refused before any attempt": {
			next: func(context.Context, resilience.Operation, func(context.Context) error) error {
				return fmt.Errorf("%w: orders", resilience.ErrBreakerOpen)
			},
			attempts: []string{"boom"},
		},
		"refused after a failed attempt": {
			next: func(ctx context.Context, _ resilience.Operation, do func(context.Context) error) error {
				_ = do(ctx)
				return refused
			},
			attempts: []string{"unavailable", "boom"},
		},
		"the failed attempt wrapped below": {
			next: func(ctx context.Context, _ resilience.Operation, do func(context.Context) error) error {
				return fmt.Errorf("%w: orders.Place: %w", resilience.ErrDeadlineExceeded, do(ctx))
			},
			attempts: []string{"unavailable"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			provider, logs := memoryLogs(slog.LevelInfo)
			_ = observe.Logging(observe.Config{LoggerProvider: provider, Category: categoryOf})(tc.next)(context.Background(), op(),
				func(context.Context) error { return categorized{"unavailable"} })

			got := logs.maps()
			if len(got) != len(tc.attempts) {
				t.Fatalf("records = %v, want %d", got, len(tc.attempts))
			}
			for i, category := range tc.attempts {
				assertRecord(t, got[i], map[string]any{
					"level": "WARN", "msg": "transport: call failed", "dmpf.retry.attempt": int64(i + 1), "error.type": category,
				})
			}
		})
	}
}

func fnd07(err error) string {
	var categorized redact.Categorized
	if errors.As(err, &categorized) {
		return categorized.ErrorCategory()
	}
	return redact.CategoryUnclassified
}

func TestLoggingRecordsABusinessRejectionAtDebugAndAFailureAtWarn(t *testing.T) {
	levels := map[string]string{
		"NotFound": "DEBUG", "Validation": "DEBUG", "DomainRejection": "DEBUG", "Conflict": "DEBUG",
		"Forbidden": "WARN", "Unauthenticated": "WARN",
		resilience.CategoryTransientDependency: "WARN", "RateLimited": "WARN", resilience.CategoryDeadlineExceeded: "WARN",
		resilience.CategoryCancelled: "WARN", "Unexpected": "WARN", redact.CategoryUnclassified: "WARN",
	}
	for category, level := range levels {
		t.Run(category, func(t *testing.T) {
			provider, logs := memoryLogs(slog.LevelDebug)
			_ = observe.Logging(observe.Config{LoggerProvider: provider, Category: fnd07})(resilience.Direct)(context.Background(), op(),
				func(context.Context) error { return categorized{category} })

			got := logs.maps()
			if len(got) != 1 {
				t.Fatalf("records = %v, want the one attempt", got)
			}
			assertRecord(t, got[0], map[string]any{
				"level": level, "msg": "transport: call failed", "dmpf.dependency": "orders", "dmpf.retry.attempt": int64(1),
				"dmpf.outcome_category": category, "error.type": category, "dmpf.error.code": "TEST-01",
			})
		})
	}
}

func TestAtInfoARepeatedNotFoundLeavesNoRecordAndAFailureStays(t *testing.T) {
	provider, logs := memoryLogs(slog.LevelInfo)
	call := observe.Logging(observe.Config{LoggerProvider: provider, Category: fnd07})(resilience.Direct)

	for range 3 {
		_ = call(context.Background(), op(), func(context.Context) error { return categorized{"NotFound"} })
	}
	_ = call(context.Background(), op(), func(context.Context) error {
		return categorized{resilience.CategoryTransientDependency}
	})

	got := logs.maps()
	if len(got) != 1 {
		t.Fatalf("records = %v, want only the failure: a NotFound is a response from the dependency, not a deviation (RF-A5)", got)
	}
	assertRecord(t, got[0], map[string]any{
		"level": "WARN", "msg": "transport: call failed", "error.type": resilience.CategoryTransientDependency,
	})
}

func TestLoggingReducesTheFailedAttemptToItsTypeAndCode(t *testing.T) {
	refusedBy := func(refusal error) resilience.Call {
		return func(context.Context, resilience.Operation, func(context.Context) error) error {
			return fmt.Errorf("%w: orders: secret detail of the refusal", refusal)
		}
	}
	cases := map[string]struct {
		category  func(error) string
		next      resilience.Call
		errorType string
		code      string
	}{
		"breaker open":       {category: fnd07, next: refusedBy(resilience.ErrBreakerOpen), errorType: resilience.CategoryTransientDependency, code: "RES-12"},
		"bulkhead saturated": {category: fnd07, next: refusedBy(resilience.ErrBulkheadSaturated), errorType: resilience.CategoryTransientDependency, code: "RES-14"},
		"the table decides the type of a categorized refusal": {
			category: func(error) string { return "Unavailable" }, next: refusedBy(resilience.ErrBreakerOpen), errorType: "Unavailable", code: "RES-12",
		},
		"not categorized": {category: fnd07, next: failing, errorType: redact.CategoryUnclassified},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			provider, logs := memoryLogs(slog.LevelInfo)
			_ = observe.Logging(observe.Config{LoggerProvider: provider, Category: tc.category})(tc.next)(context.Background(), op(),
				func(context.Context) error { return nil })

			got := logs.maps()
			if len(got) != 1 {
				t.Fatalf("records = %v, want the one refused attempt", got)
			}
			want := map[string]any{
				"level": "WARN", "msg": "transport: call failed", "dmpf.dependency": "orders", "dmpf.retry.attempt": int64(1),
				"dmpf.outcome_category": tc.errorType, "error.type": tc.errorType,
			}
			var absent []string
			if tc.code == "" {
				absent = append(absent, "dmpf.error.code")
			} else {
				want["dmpf.error.code"] = tc.code
			}
			assertRecord(t, got[0], want, absent...)
			if out := logs.text(); strings.Contains(out, "secret") || strings.Contains(out, "refused") {
				t.Fatalf("log = %q carries the error message (DAT-23)", out)
			}
		})
	}
}
