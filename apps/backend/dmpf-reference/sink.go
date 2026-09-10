package dmpfreference

import (
	"context"
	"log/slog"

	dmpfapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-app"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// Sink is the bridge of FND-06 §11 between the transport and the consumer
// adapter, plus the subscription: the channel carries every event of the
// aggregate and this consumer takes one type — the rest is acknowledged as
// received, never quarantined, because it is not addressed to it.
type Sink struct {
	Consumer  dmpfapp.Consumer
	EventType string
	Logger    *slog.Logger
}

// Handle acknowledges a delivery of another type untouched and hands
// everything else — the subscribed type and whatever does not decode — to the
// adapter, whose gesture on ack is the one the worker applies (TRP-26).
func (s Sink) Handle(ctx context.Context, raw []byte, attempt int, ack dmpfports.Acknowledger) error {
	if env, err := envelope.Unmarshal(raw); err == nil && env.Type != s.EventType {
		if s.Logger != nil {
			s.Logger.DebugContext(ctx, "delivery of another event type acknowledged", "type", env.Type, "id", env.ID)
		}
		return ack.Ack(ctx)
	}
	_, err := s.Consumer.Consume(ctx, dmpfapp.Delivery{Raw: raw, Attempt: attempt}, ack)
	return err
}
