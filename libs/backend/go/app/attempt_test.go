package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// observingHandler records what the adapter handed the handler, which is the
// only place the attempt's own context is observable.
type observingHandler struct {
	attempts  []app.Attempt
	deadlines []time.Duration
	governed  []bool
}

func (h *observingHandler) handle(ctx context.Context, _ ports.Receipt, _ envelope.Envelope) (application.Disposition, error) {
	attempt, _ := app.AttemptFrom(ctx)
	at, ok := ctx.Deadline()
	h.attempts = append(h.attempts, attempt)
	h.governed = append(h.governed, ok)
	h.deadlines = append(h.deadlines, time.Until(at))
	return application.R1D1, nil
}

func TestTheConsumerGovernsTheTimeOfEachAttempt(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	handler := &observingHandler{}
	consumer := newConsumer(&fakeHandler{}, &fakeContainment{}, 0)
	consumer.Handle = handler.handle
	consumer.Timeout = 80 * time.Millisecond

	if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v, want nil", err)
	}

	if !handler.governed[0] {
		t.Fatalf("the handler ran with no deadline: CTX-28 makes the consumer's own policy mount one")
	}
	if remaining := handler.deadlines[0]; remaining <= 0 || remaining > consumer.Timeout {
		t.Fatalf("remaining = %v, want a positive span within the declared timeout of %v", remaining, consumer.Timeout)
	}
}

func TestEachAttemptCarriesItsOwnIdentity(t *testing.T) {
	t.Parallel()
	raw, env := validRaw(t)
	handler := &observingHandler{}
	consumer := newConsumer(&fakeHandler{}, &fakeContainment{}, 0)
	consumer.Handle = handler.handle
	consumer.Timeout = time.Second

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: attempt}, &fakeAck{}); err != nil {
			t.Fatalf("Consume() attempt %d = %v, want nil", attempt, err)
		}
	}

	first, second := handler.attempts[0], handler.attempts[1]
	if first.RequestID == "" || second.RequestID == "" {
		t.Fatalf("RequestID = %q and %q, want an identifier minted for each attempt (CTX-28)", first.RequestID, second.RequestID)
	}
	if first.RequestID == second.RequestID {
		t.Fatalf("both attempts carry %q, want one identifier per attempt of processing (CTX-28)", first.RequestID)
	}
	if first.RequestID == env.ID || second.RequestID == env.ID {
		t.Fatalf("the request id equals the envelope id %q, want the consumer's own and never one read from the envelope (CTX-28)", env.ID)
	}
	if first.Number != 1 || second.Number != 2 {
		t.Fatalf("Number = %d and %d, want the transport's delivery count", first.Number, second.Number)
	}
}

func TestAConsumerWithoutATimeoutIsRefused(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	consumer := newConsumer(&fakeHandler{}, &fakeContainment{}, 0)
	consumer.Timeout = 0

	_, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{})

	if !errors.Is(err, app.ErrIncompleteConsumer) {
		t.Fatalf("got %v, want ErrIncompleteConsumer: a consumer with no time policy has no deadline to mount", err)
	}
}
