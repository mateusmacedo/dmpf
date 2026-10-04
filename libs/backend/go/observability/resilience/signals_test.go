package resilience_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

type recordingExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error   { return nil }
func (e *recordingExporter) ForceFlush(context.Context) error { return nil }

func (e *recordingExporter) snapshot() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdklog.Record(nil), e.records...)
}

const resilienceScope = "github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"

func otlpProvider() (log.LoggerProvider, *recordingExporter) {
	exporter := &recordingExporter{}
	return sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter))), exporter
}

type transitionRecord struct {
	dependency, state, scope string
	severity                 log.Severity
}

func transitionRecords(records []sdklog.Record) []transitionRecord {
	out := make([]transitionRecord, 0, len(records))
	for _, record := range records {
		r := transitionRecord{severity: record.Severity(), scope: record.InstrumentationScope().Name}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			switch kv.Key {
			case tracing.KeyDependency:
				r.dependency = kv.Value.AsString()
			case logging.KeyBreakerState:
				r.state = kv.Value.AsString()
			}
			return true
		})
		out = append(out, r)
	}
	return out
}

func spanRecorder(t *testing.T) (*tracetest.SpanRecorder, *sdktrace.TracerProvider) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})
	return recorder, provider
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

func eventAttribute(event sdktrace.Event, key string) (attribute.Value, bool) {
	for _, kv := range event.Attributes {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func TestEveryTransitionOfTheBreakerLogsAWarnWithTheDependencyAndTheState(t *testing.T) {
	provider, exporter := otlpProvider()
	fake := clock.NewFake(start)
	breaker := resilience.NewBreaker("payments", breakerPolicy(), fake, nil).LogsTo(provider)

	var outcome error
	call := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return outcome
	})

	outcome = errDependency
	for range breakerPolicy().MinSamples {
		_ = call(context.Background(), remoteOp(time.Second), nil)
	}
	fake.Advance(breakerPolicy().Cooldown)
	outcome = nil
	if err := call(context.Background(), remoteOp(time.Second), nil); err != nil {
		t.Fatalf("the probe = %v, want nil", err)
	}

	got := transitionRecords(exporter.snapshot())
	want := []transitionRecord{
		{dependency: "payments", state: "open", scope: resilienceScope, severity: log.SeverityWarn},
		{dependency: "payments", state: "half_open", scope: resilienceScope, severity: log.SeverityWarn},
		{dependency: "payments", state: "closed", scope: resilienceScope, severity: log.SeverityWarn},
	}
	if len(got) != len(want) {
		t.Fatalf("records = %+v, want exactly one per transition %+v (RF-A6)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record %d = %+v, want %+v (RF-A6)", i, got[i], want[i])
		}
	}
}

func TestACallThatChangesNoStateLogsNothing(t *testing.T) {
	provider, exporter := otlpProvider()
	breaker := resilience.NewBreaker("payments", breakerPolicy(), clock.NewFake(start), nil).LogsTo(provider)

	call := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return nil
	})
	for range breakerPolicy().MinSamples {
		_ = call(context.Background(), remoteOp(time.Second), nil)
	}

	if records := exporter.snapshot(); len(records) != 0 {
		t.Fatalf("records = %v on a closed breaker that stayed closed, want none", transitionRecords(records))
	}
}

func TestARefusalByAnOpenBreakerLeavesTheRejectedEventOnTheResilienceSpan(t *testing.T) {
	recorder, provider := spanRecorder(t)
	breaker := resilience.NewBreaker("payments", breakerPolicy(), clock.NewFake(start), nil)

	call := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return errDependency
	})
	for range breakerPolicy().MinSamples {
		_ = call(context.Background(), remoteOp(time.Second), nil)
	}

	ctx, span := provider.Tracer("observability").Start(context.Background(), "dmpf.resilience payments")
	err := call(ctx, remoteOp(time.Second), nil)
	span.End()

	if !errors.Is(err, resilience.ErrBreakerOpen) {
		t.Fatalf("call() = %v, want ErrBreakerOpen", err)
	}
	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want the resilience span", len(ended))
	}
	events := eventsNamed(ended[0], tracing.EventBreakerRejected)
	if len(events) != 1 {
		t.Fatalf("events = %v, want one %s (RF-B5)", ended[0].Events(), tracing.EventBreakerRejected)
	}
	if dependency, ok := eventAttribute(events[0], tracing.KeyDependency); !ok || dependency.AsString() != "payments" {
		t.Errorf("event %s carries %s = %v, want payments", tracing.EventBreakerRejected, tracing.KeyDependency, dependency)
	}
}

func TestACallLetThroughLeavesNoRejectedEvent(t *testing.T) {
	recorder, provider := spanRecorder(t)
	breaker := resilience.NewBreaker("payments", breakerPolicy(), clock.NewFake(start), nil)
	call := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return errDependency
	})

	ctx, span := provider.Tracer("observability").Start(context.Background(), "dmpf.resilience payments")
	_ = call(ctx, remoteOp(time.Second), nil)
	span.End()

	if events := eventsNamed(recorder.Ended()[0], tracing.EventBreakerRejected); len(events) != 0 {
		t.Fatalf("events = %v on a call the closed breaker let through", events)
	}
}

func TestADegradationLeavesTheDegradedEventWithItsCodeOnTheResilienceSpan(t *testing.T) {
	recorder, provider := spanRecorder(t)
	call := degrading(t, resilience.Degrade, nil, errDependency)

	ctx, span := provider.Tracer("observability").Start(context.Background(), "dmpf.resilience payments")
	err := call(ctx, remoteOp(time.Second), nil)
	span.End()

	var degraded *resilience.DegradedResult
	if !errors.As(err, &degraded) {
		t.Fatalf("call() = %v, want a *DegradedResult", err)
	}
	events := eventsNamed(recorder.Ended()[0], tracing.EventDegraded)
	if len(events) != 1 {
		t.Fatalf("events = %v, want one %s (RF-B5)", recorder.Ended()[0].Events(), tracing.EventDegraded)
	}
	if code, ok := eventAttribute(events[0], tracing.KeyErrorCode); !ok || code.AsString() != "RES-37" {
		t.Errorf("event %s carries %s = %v, want RES-37", tracing.EventDegraded, tracing.KeyErrorCode, code)
	}
}

func TestASuccessfulCallUnderDegradationLeavesNoDegradedEvent(t *testing.T) {
	recorder, provider := spanRecorder(t)
	call := degrading(t, resilience.Degrade, nil, nil)

	ctx, span := provider.Tracer("observability").Start(context.Background(), "dmpf.resilience payments")
	_ = call(ctx, remoteOp(time.Second), nil)
	span.End()

	if events := eventsNamed(recorder.Ended()[0], tracing.EventDegraded); len(events) != 0 {
		t.Fatalf("events = %v on a call that did not degrade", events)
	}
}
