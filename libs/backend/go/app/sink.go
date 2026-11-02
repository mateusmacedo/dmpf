package app

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Sink is the bridge of ADR-039 between the Sink of a transport, Kafka or SQS,
// and the Consumer: every delivery goes to Consume, with no filter of its own,
// because the subscription is the Consumer's Accepts (KFK-03c).
type Sink struct {
	Consumer Consumer
}

// Handle runs Consume on the delivery and returns its error; the Outcome stays
// in the adapter's signals.
func (s Sink) Handle(ctx context.Context, raw []byte, attempt int, ack ports.Acknowledger) error {
	_, err := s.Consumer.Consume(ctx, Delivery{Raw: raw, Attempt: attempt}, ack)
	return err
}
