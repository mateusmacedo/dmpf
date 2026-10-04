package relay

import (
	"context"
	"crypto/rand"
	"errors"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	sendOperation = "send"
	outcomeOK     = "ok"
)

// The transitions stay out of the send and run under the drain (RF-B7).
func (r Relay) send(ctx context.Context, record postgres.Claimed) (own, published error) {
	began := time.Now()
	ctx, span := r.openSend(ctx, record)
	defer span.End()

	message, own := assembled(record, r.Source)
	if own != nil {
		span.conclude(own)
		category := span.category(own)
		r.measureSend(ctx, began, record.Destination, category, false)
		r.logPublishFailed(ctx, record, own, category)
		return own, nil
	}
	published = r.Publisher.Publish(ctx, record.Destination, message)
	span.conclude(published)
	category := span.category(published)
	r.measureSend(ctx, began, record.Destination, category, true)
	if published != nil {
		r.logPublishFailed(ctx, record, published, category)
	}
	return nil, published
}

func assembled(record postgres.Claimed, source string) ([]byte, error) {
	env, err := Assemble(record, source)
	if err != nil {
		return nil, err
	}
	return envelope.Marshal(env)
}

// openSend starts from a context without the drain's request id: baggagecopy
// writes every parent member at OnStart and would replace the send's own
// (baggagecopy@v0.17.0/processor.go:42-50). The baggage comes after Start (RF-B8).
func (r Relay) openSend(ctx context.Context, record postgres.Claimed) (context.Context, *sendSpan) {
	metadata := metadataStrings(record.Metadata)
	var links []trace.Link
	if creation, ok := creationFrom(metadata); ok {
		links = append(links, trace.Link{SpanContext: creation})
	}

	requestID := rand.Text()
	address := r.address(record.Destination)
	ctx, span := r.tracer().Start(withoutRequestID(ctx), sendName(address),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithLinks(links...),
		trace.WithAttributes(r.sendAttributes(record, metadata, address, requestID)...))

	owned := &sendSpan{Span: span}
	ctx = tracing.WithOwnedSpan(ctx, owned)
	return withBaggage(ctx, tracing.Attributes{}.
		CorrelationID(metadata[metaCorrelationID]).
		TenantID(metadata[metaTenantID]).
		RequestID(requestID)), owned
}

func (r Relay) sendAttributes(record postgres.Claimed, metadata map[string]string, address, requestID string) []attribute.KeyValue {
	attributes := append(tracing.Attributes{}.
		TenantID(metadata[metaTenantID]).
		RequestID(requestID).
		OutboxAttempt(record.AttemptCount).
		KeyValues(),
		semconv.MessagingOperationTypeSend,
		semconv.MessagingOperationName(sendOperation),
		semconv.MessagingMessageID(record.MessageID),
		semconv.CloudEventsEventID(record.MessageID),
		semconv.CloudEventsEventSource(r.Source),
		semconv.CloudEventsEventType(record.MessageType))
	if r.System != "" {
		attributes = append(attributes, semconv.MessagingSystemKey.String(r.System))
	}
	if address != "" {
		attributes = append(attributes, semconv.MessagingDestinationName(address))
	}
	if conversation := metadata[metaCorrelationID]; conversation != "" {
		attributes = append(attributes, semconv.MessagingMessageConversationID(conversation))
	}
	return attributes
}

// RF-B7 names the send by the broker's topic; the logical destination never
// stands in for it.
func (r Relay) address(destination string) string {
	if r.Address == nil {
		return ""
	}
	return r.Address(destination)
}

func sendName(address string) string {
	if address == "" {
		return sendOperation
	}
	return sendOperation + " " + address
}

func withoutRequestID(ctx context.Context) context.Context {
	return baggage.ContextWithBaggage(ctx, baggage.FromContext(ctx).DeleteMember(tracing.KeyRequestID))
}

// sendSpan lets the transport's resilience decorator, which records on the span
// it finds through tracing.OwnsSpan, keep its finer outcome over the relay's.
type sendSpan struct {
	trace.Span
	outcomeRecorded  atomic.Bool
	recordedCategory atomic.Pointer[string]
}

func (s *sendSpan) SetAttributes(attributes ...attribute.KeyValue) {
	for _, kv := range attributes {
		switch kv.Key {
		case tracing.KeyOutcomeCategory:
			s.outcomeRecorded.Store(true)
		case semconv.ErrorTypeKey:
			category := kv.Value.AsString()
			s.recordedCategory.Store(&category)
		}
	}
	s.Span.SetAttributes(attributes...)
}

func (s *sendSpan) category(err error) string {
	if err == nil {
		return ""
	}
	if recorded := s.recordedCategory.Load(); recorded != nil {
		return *recorded
	}
	return sendCategory(err)
}

func (s *sendSpan) conclude(err error) {
	if s.outcomeRecorded.Load() {
		return
	}
	category := sendCategory(err)
	s.Span.SetAttributes(tracing.Attributes{}.OutcomeCategory(category).KeyValues()...)
	if err != nil {
		tracing.RecordError(s.Span, category)
	}
}

func sendCategory(err error) string {
	if err == nil {
		return outcomeOK
	}
	var categorized interface{ ErrorCategory() string }
	if errors.As(err, &categorized) && categorized.ErrorCategory() != "" {
		return categorized.ErrorCategory()
	}
	return semconv.ErrorTypeOther.Value.AsString()
}
