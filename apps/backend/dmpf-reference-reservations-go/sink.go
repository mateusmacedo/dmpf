package dmpfreferencereservations

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	dmpfapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-app"
	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/tracing"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// Sink is the bridge of FND-06 §11 between the transport and the consumer
// adapter, plus the subscription: the channel carries every event of the
// aggregate and this consumer takes one type — the rest is acknowledged as
// received, never quarantined, because it is not addressed to it.
type Sink struct {
	Consumer  dmpfapp.Consumer
	EventType string
	Logger    *slog.Logger
	Tracer    trace.Tracer
}

// Handle acknowledges a delivery of another type untouched and hands everything
// else to the adapter under a consume span continuing the envelope's trace (TRC-07).
func (s Sink) Handle(ctx context.Context, raw []byte, attempt int, ack dmpfports.Acknowledger) error {
	env, err := envelope.Unmarshal(raw)
	if err == nil && env.Type != s.EventType {
		if s.Logger != nil {
			s.Logger.DebugContext(ctx, "delivery of another event type acknowledged", "type", env.Type, "id", env.ID)
		}
		return ack.Ack(ctx)
	}
	if err == nil {
		ctx = propagation.TraceContext{}.Extract(ctx, traceCarrier(env))
	}

	ctx, span := s.tracer().Start(ctx, "dmpf.kafka.consume "+s.EventType, trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()

	outcome, err := s.Consumer.Consume(ctx, dmpfapp.Delivery{Raw: raw, Attempt: attempt}, ack)
	if err != nil {
		// WHY: R1D3 is the healthy retry path and still returns a non-nil error
		// joined with the release, so marking every error would report the
		// normal redelivery as a fault on every first attempt.
		if outcome.Disposition != dmpfapplication.R1D3 {
			tracing.RecordError(span, "consume")
		}
		return err
	}
	return nil
}

func (s Sink) tracer() trace.Tracer {
	if s.Tracer == nil {
		return otel.GetTracerProvider().Tracer("dmpf-reference-reservations")
	}
	return s.Tracer
}

func traceCarrier(env envelope.Envelope) propagation.MapCarrier {
	carrier := propagation.MapCarrier{"traceparent": env.TraceParent}
	if env.TraceState != nil {
		carrier["tracestate"] = *env.TraceState
	}
	return carrier
}
