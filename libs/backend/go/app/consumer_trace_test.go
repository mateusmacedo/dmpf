package app_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	testTopic = "dmpf.orders.events.v1"
	testGroup = "reservations-orders"

	keyContainmentReason = "dmpf.containment.reason"
	keyProvenance        = "dmpf.provenance.traceparent"
)

var (
	creationTrace = mustTraceID("0af7651916cd43dd8448eb211c80319c")
	creationSpan  = mustSpanID("b7ad6b7169203331")
)

func mustTraceID(text string) trace.TraceID {
	id, err := trace.TraceIDFromHex(text)
	if err != nil {
		panic(err)
	}
	return id
}

func mustSpanID(text string) trace.SpanID {
	id, err := trace.SpanIDFromHex(text)
	if err != nil {
		panic(err)
	}
	return id
}

// baggageCopier stands in for baggagecopy@v0.17.0, which copies at OnStart.
type baggageCopier struct{}

func (baggageCopier) OnStart(parent context.Context, span sdktrace.ReadWriteSpan) {
	for _, member := range baggage.FromContext(parent).Members() {
		span.SetAttributes(attribute.String(member.Key(), member.Value()))
	}
}
func (baggageCopier) OnEnd(sdktrace.ReadOnlySpan)      {}
func (baggageCopier) Shutdown(context.Context) error   { return nil }
func (baggageCopier) ForceFlush(context.Context) error { return nil }

// samplerSpy keeps what the SDK hands the sampler at the root's creation. The
// decision on those inputs is otelboot's, proven by its own sampler tests.
type samplerSpy struct {
	seen      []sdktrace.SamplingParameters
	delegated sdktrace.Sampler
}

func (s *samplerSpy) ShouldSample(parameters sdktrace.SamplingParameters) sdktrace.SamplingResult {
	s.seen = append(s.seen, parameters)
	return s.delegated.ShouldSample(parameters)
}

func (s *samplerSpy) Description() string { return "samplerSpy" }

func tracedConsumer(handle app.Handler, containment ports.Containment, sampler sdktrace.Sampler) (app.Consumer, trace.Tracer, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithSpanProcessor(baggageCopier{}),
		sdktrace.WithSpanProcessor(recorder),
	)
	tracer := provider.Tracer("consumer-test")
	consumer := newConsumer(&fakeHandler{}, &fakeContainment{}, 3)
	consumer.Handle = handle
	consumer.Containment = containment
	consumer.Tracer = tracer
	consumer.System = "kafka"
	consumer.Channel = app.Channel{Address: testTopic, Group: testGroup}
	return consumer, tracer, recorder
}

func processes(recorder *tracetest.SpanRecorder) []sdktrace.ReadOnlySpan {
	var found []sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.SpanKind() == trace.SpanKindConsumer {
			found = append(found, span)
		}
	}
	return found
}

func onlyProcess(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()

	found := processes(recorder)
	if len(found) != 1 {
		t.Fatalf("%d consumer spans among %d ended, want exactly the process", len(found), len(recorder.Ended()))
	}
	return found[0]
}

func attributeOf(span sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func requireAttributes(t *testing.T, span sdktrace.ReadOnlySpan, want map[string]attribute.Value) {
	t.Helper()

	for key, value := range want {
		got, ok := attributeOf(span, key)
		if !ok {
			t.Errorf("%q carries no %s, want %v", span.Name(), key, value.String())
			continue
		}
		if got != value {
			t.Errorf("%q %s = %v, want %v", span.Name(), key, got.String(), value.String())
		}
	}
}

func requireAbsent(t *testing.T, span sdktrace.ReadOnlySpan, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if got, ok := attributeOf(span, key); ok {
			t.Errorf("%q carries %s = %v, want it absent", span.Name(), key, got.String())
		}
	}
}

func underACallerSpan(t *testing.T, tracer trace.Tracer) context.Context {
	t.Helper()

	member, err := baggage.NewMemberRaw(tracing.KeyCorrelationID, "caller-corr")
	if err != nil {
		t.Fatalf("NewMemberRaw() = %v", err)
	}
	bag, err := baggage.New(member)
	if err != nil {
		t.Fatalf("baggage.New() = %v", err)
	}
	ctx, span := tracer.Start(baggage.ContextWithBaggage(context.Background(), bag), "caller")
	t.Cleanup(func() { span.End() })
	return ctx
}

func TestTheProcessIsAConsumerRootLinkedToTheCreationContext(t *testing.T) {
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}
	consumer, tracer, recorder := tracedConsumer(handler.handle, &fakeContainment{}, sdktrace.AlwaysSample())

	if _, err := consumer.Consume(underACallerSpan(t, tracer), app.Delivery{Raw: raw, Attempt: 2}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	process := onlyProcess(t, recorder)

	if process.Name() != "process "+testTopic {
		t.Fatalf("name = %q, want %q", process.Name(), "process "+testTopic)
	}
	if process.Parent().IsValid() {
		t.Fatalf("parent = %v, want a new root", process.Parent())
	}
	links := process.Links()
	if len(links) != 1 {
		t.Fatalf("%d links, want the creation context only", len(links))
	}
	if links[0].SpanContext.TraceID() != creationTrace || links[0].SpanContext.SpanID() != creationSpan {
		t.Fatalf("link = %v, want the creation context %s/%s", links[0].SpanContext, creationTrace, creationSpan)
	}
	requireAttributes(t, process, map[string]attribute.Value{
		string(semconv.MessagingSystemKey):                attribute.StringValue("kafka"),
		string(semconv.MessagingOperationTypeKey):         attribute.StringValue("process"),
		string(semconv.MessagingOperationNameKey):         attribute.StringValue("process"),
		string(semconv.MessagingDestinationNameKey):       attribute.StringValue(testTopic),
		string(semconv.MessagingConsumerGroupNameKey):     attribute.StringValue(testGroup),
		string(semconv.MessagingMessageIDKey):             attribute.StringValue("evt-1"),
		string(semconv.MessagingMessageConversationIDKey): attribute.StringValue("corr-1"),
		string(semconv.CloudEventsEventIDKey):             attribute.StringValue("evt-1"),
		string(semconv.CloudEventsEventSourceKey):         attribute.StringValue("urn:dmpf:orders"),
		string(semconv.CloudEventsEventTypeKey):           attribute.StringValue("com.company.orders.order-placed.v1"),
		tracing.KeyTenantID:                               attribute.StringValue(fixtureTenant),
		tracing.KeyRequestID:                              attribute.StringValue(handler.execution.RequestID()),
		tracing.KeyInboxAttempt:                           attribute.IntValue(2),
		tracing.KeyTrafficClass:                           attribute.StringValue(string(tracing.ClassWrite)),
	})
	requireAbsent(t, process, tracing.KeyCorrelationID, keyContainmentReason, keyProvenance)
}

func TestTheProcessGroupIsTheChannelsNotTheInboxConsumerName(t *testing.T) {
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}
	consumer, _, recorder := tracedConsumer(handler.handle, &fakeContainment{}, sdktrace.AlwaysSample())

	if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}

	group, _ := attributeOf(onlyProcess(t, recorder), string(semconv.MessagingConsumerGroupNameKey))
	if group.AsString() == consumerName {
		t.Fatalf("%s = %q, the inbox consumer name; want the channel group %q", semconv.MessagingConsumerGroupNameKey, group.AsString(), testGroup)
	}
}

func TestTheProcessHandsTheSamplerItsClassAndTheCreationContext(t *testing.T) {
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}
	spy := &samplerSpy{delegated: sdktrace.AlwaysSample()}
	consumer, tracer, _ := tracedConsumer(handler.handle, &fakeContainment{}, spy)

	if _, err := consumer.Consume(underACallerSpan(t, tracer), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}

	parameters := samplingOf(t, spy, trace.SpanKindConsumer)
	if trace.SpanContextFromContext(parameters.ParentContext).IsValid() {
		t.Fatal("the sampler saw a parent, want a new root")
	}
	if class := classOf(parameters); class != string(tracing.ClassWrite) {
		t.Fatalf("sampler saw %s = %q, want %q", tracing.KeyTrafficClass, class, tracing.ClassWrite)
	}
	if len(parameters.Links) != 1 || !parameters.Links[0].SpanContext.IsSampled() || parameters.Links[0].SpanContext.TraceID() != creationTrace {
		t.Fatalf("sampler saw links %v, want the sampled creation context", parameters.Links)
	}
}

func samplingOf(t *testing.T, spy *samplerSpy, kind trace.SpanKind) sdktrace.SamplingParameters {
	t.Helper()

	var found []sdktrace.SamplingParameters
	for _, parameters := range spy.seen {
		if parameters.Kind == kind {
			found = append(found, parameters)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the sampler was asked %d times for a %s span, want once", len(found), kind)
	}
	return found[0]
}

func classOf(parameters sdktrace.SamplingParameters) string {
	for _, kv := range parameters.Attributes {
		if string(kv.Key) == tracing.KeyTrafficClass {
			return kv.Value.AsString()
		}
	}
	return ""
}

func TestARefusedBoundaryOpensAnErrorRootLinkedToTheReceivedContext(t *testing.T) {
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}
	containment := &fakeContainment{}
	spy := &samplerSpy{delegated: sdktrace.AlwaysSample()}
	consumer, tracer, recorder := tracedConsumer(handler.handle, containment, spy)
	consumer.Boundary.Sources = []string{"urn:dmpf:someone-else"}

	if _, err := consumer.Consume(underACallerSpan(t, tracer), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	process := onlyProcess(t, recorder)

	if handler.calls != 0 || len(containment.contained) != 1 {
		t.Fatalf("handler calls = %d, contained = %d; want the message contained unhandled", handler.calls, len(containment.contained))
	}
	if process.Parent().IsValid() {
		t.Fatalf("parent = %v, want a new root", process.Parent())
	}
	if len(process.Links()) != 1 || process.Links()[0].SpanContext.TraceID() != creationTrace {
		t.Fatalf("links = %v, want the received context", process.Links())
	}
	requireAttributes(t, process, map[string]attribute.Value{
		tracing.KeyTrafficClass:                   attribute.StringValue(string(tracing.ClassError)),
		keyContainmentReason:                      attribute.StringValue(string(ports.ReasonUntrustedBoundary)),
		string(semconv.MessagingMessageIDKey):     attribute.StringValue("evt-1"),
		string(semconv.MessagingSystemKey):        attribute.StringValue("kafka"),
		string(semconv.MessagingOperationTypeKey): attribute.StringValue("process"),
	})
	requireAbsent(t, process, keyProvenance, tracing.KeyCorrelationID, tracing.KeyTenantID, string(semconv.MessagingMessageConversationIDKey))

	parameters := samplingOf(t, spy, trace.SpanKindConsumer)
	if trace.SpanContextFromContext(parameters.ParentContext).IsValid() {
		t.Fatal("the sampler saw a parent, want a new root")
	}
	if class := classOf(parameters); class != string(tracing.ClassError) {
		t.Fatalf("sampler saw %s = %q, want %q", tracing.KeyTrafficClass, class, tracing.ClassError)
	}
}

func TestAnInvalidEnvelopeOpensNoProcess(t *testing.T) {
	handler := &fakeHandler{disposition: application.R1D1}
	consumer, _, recorder := tracedConsumer(handler.handle, &fakeContainment{}, sdktrace.AlwaysSample())

	if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: []byte("not an envelope"), Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	if found := processes(recorder); len(found) != 0 {
		t.Fatalf("%d process spans, want none before the boundary has an envelope to judge", len(found))
	}
}

func TestTheHandlerRunsUnderTheProcessWithTheExecutionBaggage(t *testing.T) {
	raw, _ := validRaw(t)
	var under trace.SpanContext
	var bag baggage.Baggage
	var tracer trace.Tracer
	handle := func(ctx context.Context, _ ports.Receipt, _ envelope.Envelope) (application.Disposition, error) {
		under = trace.SpanContextFromContext(ctx)
		bag = baggage.FromContext(ctx)
		_, child := tracer.Start(ctx, "child")
		child.End()
		return application.R1D1, nil
	}
	consumer, tracer, recorder := tracedConsumer(handle, &fakeContainment{}, sdktrace.AlwaysSample())

	if _, err := consumer.Consume(underACallerSpan(t, tracer), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	process := onlyProcess(t, recorder)
	requestID, _ := attributeOf(process, tracing.KeyRequestID)

	if !under.Equal(process.SpanContext()) {
		t.Fatalf("handler ran under %v, want the process %v", under, process.SpanContext())
	}
	for key, want := range map[string]string{
		tracing.KeyCorrelationID: "corr-1",
		tracing.KeyRequestID:     requestID.AsString(),
		tracing.KeyTenantID:      fixtureTenant,
	} {
		if got := bag.Member(key).Value(); got != want {
			t.Errorf("handler baggage %s = %q, want %q", key, got, want)
		}
	}

	var child sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.Name() == "child" {
			child = span
		}
	}
	if child == nil {
		t.Fatal("the handler's child span did not end")
	}
	requireAttributes(t, child, map[string]attribute.Value{
		tracing.KeyCorrelationID: attribute.StringValue("corr-1"),
		tracing.KeyRequestID:     requestID,
		tracing.KeyTenantID:      attribute.StringValue(fixtureTenant),
	})
	requireAbsent(t, process, tracing.KeyCorrelationID)
}

type contextAck struct{ ctx context.Context }

func (a *contextAck) Ack(ctx context.Context) error {
	a.ctx = ctx
	return nil
}

func (a *contextAck) Release(ctx context.Context) error {
	a.ctx = ctx
	return nil
}

func TestTheGestureRunsUnderTheProcessWithTheExecutionBaggage(t *testing.T) {
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}
	consumer, tracer, recorder := tracedConsumer(handler.handle, &fakeContainment{}, sdktrace.AlwaysSample())
	ack := &contextAck{}

	if _, err := consumer.Consume(underACallerSpan(t, tracer), app.Delivery{Raw: raw, Attempt: 1}, ack); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	process := onlyProcess(t, recorder)

	if under := trace.SpanContextFromContext(ack.ctx); !under.Equal(process.SpanContext()) {
		t.Fatalf("the ack ran under %v, want the process %v", under, process.SpanContext())
	}
	if got := baggage.FromContext(ack.ctx).Member(tracing.KeyCorrelationID).Value(); got != "corr-1" {
		t.Fatalf("ack baggage %s = %q, want %q", tracing.KeyCorrelationID, got, "corr-1")
	}
}

func TestTheProcessRecordsTheOutcome(t *testing.T) {
	transient := application.NewFailure(application.TransientDependency, true, errHandler)
	terminal := application.NewFailure(application.Unexpected, false, errHandler)
	rejection := application.NewFailure(application.DomainRejection, false, errHandler)
	invalid := application.NewFailure(application.Validation, false, errHandler)
	errBroker := errors.New("broker 10.0.0.9:9092 refused the ack")

	for _, tc := range []struct {
		name        string
		disposition application.Disposition
		handleErr   error
		ackErr      error
		attempt     int
		want        map[string]attribute.Value
		absent      []string
		failed      bool
	}{
		{
			name:        "acknowledged",
			disposition: application.R1D1,
			attempt:     1,
			want: map[string]attribute.Value{
				tracing.KeyInboxDisposition: attribute.StringValue(application.R1D1.String()),
				tracing.KeyInboxGesture:     attribute.StringValue("ack"),
				tracing.KeyOutcomeCategory:  attribute.StringValue("ok"),
			},
			absent: []string{keyContainmentReason, string(semconv.ErrorTypeKey)},
		},
		{
			name:        "released for another attempt",
			disposition: application.R1D3,
			handleErr:   transient,
			attempt:     1,
			want: map[string]attribute.Value{
				tracing.KeyInboxDisposition: attribute.StringValue(application.R1D3.String()),
				tracing.KeyInboxGesture:     attribute.StringValue("release"),
				tracing.KeyOutcomeCategory:  attribute.StringValue(string(application.TransientDependency)),
			},
			absent: []string{keyContainmentReason, string(semconv.ErrorTypeKey)},
		},
		{
			name:        "contained after the last attempt",
			disposition: application.R1D3,
			handleErr:   transient,
			attempt:     3,
			want: map[string]attribute.Value{
				tracing.KeyInboxDisposition:  attribute.StringValue(application.R1D3.String()),
				tracing.KeyInboxGesture:      attribute.StringValue("ack"),
				keyContainmentReason:         attribute.StringValue(string(ports.ReasonAttemptsExhausted)),
				tracing.KeyOutcomeCategory:   attribute.StringValue(string(application.TransientDependency)),
				string(semconv.ErrorTypeKey): attribute.StringValue(string(application.TransientDependency)),
			},
			failed: true,
		},
		{
			name:        "contained as terminal",
			disposition: application.R1D4,
			handleErr:   terminal,
			attempt:     1,
			want: map[string]attribute.Value{
				tracing.KeyInboxDisposition:  attribute.StringValue(application.R1D4.String()),
				tracing.KeyInboxGesture:      attribute.StringValue("ack"),
				keyContainmentReason:         attribute.StringValue(string(ports.ReasonTerminalFailure)),
				tracing.KeyOutcomeCategory:   attribute.StringValue(string(application.Unexpected)),
				string(semconv.ErrorTypeKey): attribute.StringValue(string(application.Unexpected)),
			},
			failed: true,
		},
		{
			name:        "a domain rejection contained as terminal",
			disposition: application.R1D4,
			handleErr:   rejection,
			attempt:     1,
			want: map[string]attribute.Value{
				tracing.KeyInboxDisposition:  attribute.StringValue(application.R1D4.String()),
				tracing.KeyInboxGesture:      attribute.StringValue("ack"),
				keyContainmentReason:         attribute.StringValue(string(ports.ReasonTerminalFailure)),
				tracing.KeyOutcomeCategory:   attribute.StringValue(string(application.DomainRejection)),
				string(semconv.ErrorTypeKey): attribute.StringValue(string(application.DomainRejection)),
			},
			failed: true,
		},
		{
			name:        "a validation contained as terminal",
			disposition: application.R1D4,
			handleErr:   invalid,
			attempt:     1,
			want: map[string]attribute.Value{
				tracing.KeyInboxDisposition:  attribute.StringValue(application.R1D4.String()),
				tracing.KeyInboxGesture:      attribute.StringValue("ack"),
				keyContainmentReason:         attribute.StringValue(string(ports.ReasonTerminalFailure)),
				tracing.KeyOutcomeCategory:   attribute.StringValue(string(application.Validation)),
				string(semconv.ErrorTypeKey): attribute.StringValue(string(application.Validation)),
			},
			failed: true,
		},
		{
			name:        "the broker refused the ack",
			disposition: application.R1D1,
			ackErr:      errBroker,
			attempt:     1,
			want: map[string]attribute.Value{
				tracing.KeyInboxGesture:      attribute.StringValue("ack"),
				tracing.KeyOutcomeCategory:   semconv.ErrorTypeOther.Value,
				string(semconv.ErrorTypeKey): semconv.ErrorTypeOther.Value,
			},
			failed: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := validRaw(t)
			handler := &fakeHandler{disposition: tc.disposition, err: tc.handleErr}
			consumer, _, recorder := tracedConsumer(handler.handle, &fakeContainment{}, sdktrace.AlwaysSample())

			_, _ = consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: tc.attempt}, &fakeAck{err: tc.ackErr})
			process := onlyProcess(t, recorder)

			requireAttributes(t, process, tc.want)
			requireAbsent(t, process, tc.absent...)
			if failed := process.Status().Code == codes.Error; failed != tc.failed {
				t.Fatalf("status = %v, want failed %v", process.Status(), tc.failed)
			}
			if process.Status().Description != "" {
				t.Fatalf("status description = %q, want none (TRC-12)", process.Status().Description)
			}
		})
	}
}

func TestAConsumerWithoutATracerStillConsumes(t *testing.T) {
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}
	ack := &fakeAck{}

	if _, err := newConsumer(handler, &fakeContainment{}, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, ack); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	if handler.calls != 1 || ack.acks != 1 {
		t.Fatalf("handler calls = %d, acks = %d; want one of each", handler.calls, ack.acks)
	}
}
