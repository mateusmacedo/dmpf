package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var _ kafka.Sink = app.Sink{}

type attemptHandler struct {
	fakeHandler
	attempt app.Attempt
}

func (h *attemptHandler) handle(ctx context.Context, r ports.Receipt, env envelope.Envelope) (application.Disposition, error) {
	h.attempt, _ = app.AttemptFrom(ctx)
	return h.fakeHandler.handle(ctx, r, env)
}

func TestTheSinkHandsTheSubscribedTypeToConsume(t *testing.T) {
	raw, _ := validRaw(t)
	handler := &attemptHandler{fakeHandler: fakeHandler{disposition: application.R1D1}}
	containment, ack := &fakeContainment{}, &fakeAck{}
	consumer := newConsumer(&handler.fakeHandler, containment, 3)
	consumer.Handle = handler.handle

	if err := (app.Sink{Consumer: consumer}).Handle(context.Background(), raw, 2, ack); err != nil {
		t.Fatalf("Handle() = %v, want nil", err)
	}

	if handler.calls != 1 || ack.acks != 1 || len(containment.contained) != 0 {
		t.Fatalf("handler=%d acks=%d contained=%d, want the adapter to run and ack once", handler.calls, ack.acks, len(containment.contained))
	}
	if handler.attempt.Number != 2 {
		t.Fatalf("attempt = %d, want the transport's delivery count 2", handler.attempt.Number)
	}
}

func TestTheSinkLeavesAnInvalidEnvelopeToConsume(t *testing.T) {
	handler, containment, ack := &fakeHandler{}, &fakeContainment{}, &fakeAck{}

	if err := (app.Sink{Consumer: newConsumer(handler, containment, 3)}).Handle(context.Background(), []byte("not a cloudevent"), 1, ack); err != nil {
		t.Fatalf("Handle() = %v, want nil", err)
	}

	if handler.calls != 0 || len(containment.contained) != 1 || ack.acks != 1 {
		t.Fatalf("handler=%d contained=%d acks=%d, want the adapter to quarantine it (INB-10)", handler.calls, len(containment.contained), ack.acks)
	}
}

func TestTheSinkHasNoFilterOfItsOwn(t *testing.T) {
	for _, eventType := range []string{itemAddedV1, "com.company.orders.order-placed.v2"} {
		t.Run(eventType, func(t *testing.T) {
			raw, _ := rawOfType(t, eventType)
			viaSink, viaConsume := &fakeContainment{}, &fakeContainment{}
			sinkAck, consumeAck := &fakeAck{}, &fakeAck{}

			sinkErr := (app.Sink{Consumer: newConsumer(&fakeHandler{}, viaSink, 3)}).Handle(context.Background(), raw, 1, sinkAck)
			_, consumeErr := newConsumer(&fakeHandler{}, viaConsume, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, consumeAck)

			if !errors.Is(sinkErr, consumeErr) || len(viaSink.contained) != len(viaConsume.contained) || *sinkAck != *consumeAck {
				t.Fatalf("sink: err=%v contained=%d ack=%+v; consume: err=%v contained=%d ack=%+v; want the same",
					sinkErr, len(viaSink.contained), *sinkAck, consumeErr, len(viaConsume.contained), *consumeAck)
			}
		})
	}
}

func TestTheSinkReturnsTheErrorOfConsume(t *testing.T) {
	raw, _ := validRaw(t)
	consumer := newConsumer(&fakeHandler{}, &fakeContainment{}, 3)
	consumer.Accepts = nil

	if err := (app.Sink{Consumer: consumer}).Handle(context.Background(), raw, 1, &fakeAck{}); !errors.Is(err, app.ErrIncompleteConsumer) {
		t.Fatalf("Handle() = %v, want %v", err, app.ErrIncompleteConsumer)
	}
}
