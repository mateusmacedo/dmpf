package app

import (
	"context"
	"log/slog"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Sink is the bridge of FND-06 §11 between the transport and the consumer
// adapter, plus the subscription: the channel carries every event of the
// aggregate and this consumer takes one type — the rest is acknowledged as
// received, never quarantined, because it is not addressed to it.
type Sink struct {
	Consumer  kernelapp.Consumer
	EventType string
	Logger    *slog.Logger
}

// Handle acknowledges a delivery of another type untouched and hands everything
// else to the adapter, which opens the process span (RF-B9).
func (s Sink) Handle(ctx context.Context, raw []byte, attempt int, ack ports.Acknowledger) error {
	env, err := envelope.Unmarshal(raw)
	if err == nil && env.Type != s.EventType {
		if s.Logger != nil {
			s.Logger.DebugContext(ctx, "delivery of another event type acknowledged",
				string(semconv.CloudEventsEventTypeKey), env.Type, string(semconv.MessagingMessageIDKey), env.ID)
		}
		return ack.Ack(ctx)
	}
	_, err = s.Consumer.Consume(ctx, kernelapp.Delivery{Raw: raw, Attempt: attempt}, ack)
	return err
}
