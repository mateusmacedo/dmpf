package app

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	processOperation     = "process"
	outcomeOK            = "ok"
	gestureAck           = "ack"
	gestureRelease       = "release"
	keyContainmentReason = "dmpf.containment.reason"
	keyPanicType         = "dmpf.panic.type"
)

func (c Consumer) tracer() trace.Tracer {
	if c.Tracer == nil {
		return noop.NewTracerProvider().Tracer("")
	}
	return c.Tracer
}

// Outside the boundary the root is of class error, which keeps the received
// link out of the sampling decision (TRC-07, RF-E5).
func (c Consumer) openProcess(ctx context.Context, env envelope.Envelope, delivery int, trusted bool, requestID string) (context.Context, trace.Span) {
	var links []trace.Link
	if received, ok := receivedContext(env); ok {
		links = []trace.Link{{SpanContext: received}}
	}

	class := tracing.ClassError
	own := tracing.Attributes{}
	if trusted {
		class = tracing.ClassWrite
		own = own.RequestID(requestID).TenantID(tenantOf(env))
	}
	attributes := append(own.TrafficClass(string(class)).KeyValues(), c.messagingAttributes(env, delivery, trusted)...)

	return c.tracer().Start(withoutExecutionBaggage(ctx), processName(c.Channel.Address),
		trace.WithNewRoot(),
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithLinks(links...),
		trace.WithAttributes(attributes...))
}

func (c Consumer) messagingAttributes(env envelope.Envelope, delivery int, trusted bool) []attribute.KeyValue {
	attributes := []attribute.KeyValue{
		semconv.MessagingOperationTypeProcess,
		semconv.MessagingOperationName(processOperation),
		semconv.MessagingMessageID(env.ID),
		semconv.CloudEventsEventID(env.ID),
		semconv.CloudEventsEventSource(env.Source),
		semconv.CloudEventsEventType(env.Type),
		attribute.Int(tracing.KeyInboxAttempt, delivery),
	}
	if c.System != "" {
		attributes = append(attributes, semconv.MessagingSystemKey.String(c.System))
	}
	if c.Channel.Address != "" {
		attributes = append(attributes, semconv.MessagingDestinationName(c.Channel.Address))
	}
	if c.Channel.Group != "" {
		attributes = append(attributes, semconv.MessagingConsumerGroupName(c.Channel.Group))
	}
	if trusted && env.CorrelationID != "" {
		attributes = append(attributes, semconv.MessagingMessageConversationID(env.CorrelationID))
	}
	return attributes
}

func processName(address string) string {
	if address == "" {
		return processOperation
	}
	return processOperation + " " + address
}

func tenantOf(env envelope.Envelope) string {
	if env.TenantID == nil {
		return ""
	}
	return *env.TenantID
}

func receivedContext(env envelope.Envelope) (trace.SpanContext, bool) {
	carrier := propagation.MapCarrier{"traceparent": env.TraceParent}
	if env.TraceState != nil {
		carrier["tracestate"] = *env.TraceState
	}
	received := trace.SpanContextFromContext(propagation.TraceContext{}.Extract(context.Background(), carrier))
	return received, received.IsValid()
}

// baggagecopy writes every member of the parent's baggage at OnStart; the
// process must not inherit the caller's execution (RF-B8, decision 3).
func withoutExecutionBaggage(ctx context.Context) context.Context {
	bag := baggage.FromContext(ctx)
	for _, key := range tracing.ExecutionBaggageKeys {
		bag = bag.DeleteMember(key)
	}
	return baggage.ContextWithBaggage(ctx, bag)
}

type gestures struct {
	ports.Acknowledger
	applied string
	err     error
}

func (g *gestures) Ack(ctx context.Context) (err error) {
	defer recoverInto(&err)
	g.applied, g.err = gestureAck, g.Acknowledger.Ack(ctx)
	return g.err
}

func (g *gestures) Release(ctx context.Context) (err error) {
	defer recoverInto(&err)
	g.applied, g.err = gestureRelease, g.Acknowledger.Release(ctx)
	return g.err
}

func conclude(span trace.Span, outcome Outcome, gesture *gestures, err error) {
	var attributes []attribute.KeyValue
	if outcome.Classified {
		attributes = append(attributes, attribute.String(tracing.KeyInboxDisposition, outcome.Disposition.String()))
	}
	if gesture.applied != "" {
		attributes = append(attributes, attribute.String(tracing.KeyInboxGesture, gesture.applied))
	}
	if outcome.Contained {
		attributes = append(attributes, attribute.String(keyContainmentReason, string(outcome.Reason)))
	}
	if goType := panicTypeOf(err); goType != "" {
		attributes = append(attributes, attribute.String(keyPanicType, goType))
	}
	category := outcomeCategory(err)
	span.SetAttributes(append(attributes, tracing.Attributes{}.OutcomeCategory(category).KeyValues()...)...)
	if failed(outcome, gesture, err) {
		tracing.RecordError(span, category)
	}
}

func failed(outcome Outcome, gesture *gestures, err error) bool {
	return err != nil && !redelivered(outcome, gesture)
}

// R1×D3 released is the healthy retry path, yet it returns the handler's error;
// marking it would report every normal redelivery as a fault.
func redelivered(outcome Outcome, gesture *gestures) bool {
	return outcome.Disposition == application.R1D3 && !outcome.Contained &&
		gesture.applied == gestureRelease && gesture.err == nil
}

func outcomeCategory(err error) string {
	if err == nil {
		return outcomeOK
	}
	if errors.Is(err, errPanicked) {
		return errPanicked.ErrorCategory()
	}
	var categorized interface{ ErrorCategory() string }
	if errors.As(err, &categorized) && categorized.ErrorCategory() != "" {
		return categorized.ErrorCategory()
	}
	return semconv.ErrorTypeOther.Value.AsString()
}
