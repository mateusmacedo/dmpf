package relay

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const drainName = "outbox drain"

func (r Relay) tracer() trace.Tracer {
	if r.Tracer == nil {
		return noop.NewTracerProvider().Tracer("")
	}
	return r.Tracer
}

// openDrain roots the scan in a trace of its own, linked to every creation
// context (RF-B7). The links go in at Start because the sampler only sees them
// there, and the baggage comes after it, as RF-B8 orders.
func (r Relay) openDrain(ctx context.Context, claimedAt time.Time, claimID string, claimed []postgres.Claimed) (context.Context, trace.Span) {
	links := make([]trace.Link, 0, len(claimed))
	invalid := 0
	for _, record := range claimed {
		creation, ok := creationContext(record.Metadata)
		if !ok {
			invalid++
			continue
		}
		links = append(links, trace.Link{
			SpanContext: creation,
			Attributes:  []attribute.KeyValue{semconv.MessagingMessageID(record.MessageID)},
		})
	}

	attributes := append(tracing.Attributes{}.
		TrafficClass(string(tracing.ClassWrite)).
		OutboxClaimID(claimID).
		KeyValues(), semconv.MessagingBatchMessageCount(len(claimed)))

	ctx, span := r.tracer().Start(ctx, drainSpanName(claimed),
		trace.WithNewRoot(),
		trace.WithTimestamp(claimedAt),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithLinks(links...),
		trace.WithAttributes(attributes...))

	tracing.Claimed(span)
	for range invalid {
		tracing.InvalidCreationContext(span)
	}

	requestID := rand.Text()
	span.SetAttributes(tracing.Attributes{}.RequestID(requestID).KeyValues()...)
	return withBaggage(ctx, tracing.Attributes{}.RequestID(requestID)), span
}

func drainSpanName(claimed []postgres.Claimed) string {
	for _, record := range claimed[1:] {
		if record.Destination != claimed[0].Destination {
			return drainName
		}
	}
	return drainName + " " + claimed[0].Destination
}

func withBaggage(ctx context.Context, attributes tracing.Attributes) context.Context {
	bag := baggage.FromContext(ctx)
	for _, kv := range attributes.KeyValues() {
		member, err := baggage.NewMemberRaw(string(kv.Key), kv.Value.AsString())
		if err != nil {
			continue
		}
		if next, err := bag.SetMember(member); err == nil {
			bag = next
		}
	}
	return baggage.ContextWithBaggage(ctx, bag)
}

func creationContext(metadata []byte) (trace.SpanContext, bool) {
	return creationFrom(metadataStrings(metadata))
}

// creationFrom reads the context the producer wrote, never failing the drain
// on it: an unreadable one only costs the link (RF-B7).
func creationFrom(metadata map[string]string) (trace.SpanContext, bool) {
	carrier := propagation.MapCarrier{}
	for _, key := range [...]string{metaTraceParent, metaTraceState} {
		if text, ok := metadata[key]; ok {
			carrier[key] = text
		}
	}

	creation := trace.SpanContextFromContext(propagation.TraceContext{}.Extract(context.Background(), carrier))
	return creation, creation.IsValid()
}

func metadataStrings(metadata []byte) map[string]string {
	var raw map[string]json.RawMessage
	if json.Unmarshal(metadata, &raw) != nil {
		return nil
	}
	values := make(map[string]string, len(raw))
	for key, value := range raw {
		var text string
		if json.Unmarshal(value, &text) == nil {
			values[key] = text
		}
	}
	return values
}
