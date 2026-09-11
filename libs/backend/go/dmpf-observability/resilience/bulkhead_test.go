package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/tracing"
)

// barrier is the deterministic occupancy fixture: each admitted call reports
// that it entered and then blocks until the test releases it, so the pool and
// the queue are held in a known state without any sleeping.
type barrier struct {
	entered chan struct{}
	release chan struct{}
}

func newBarrier(capacity int) *barrier {
	return &barrier{entered: make(chan struct{}, capacity), release: make(chan struct{})}
}

func (b *barrier) hold(context.Context, resilience.Operation, func(context.Context) error) error {
	b.entered <- struct{}{}
	<-b.release
	return nil
}

func (b *barrier) awaitEntered(t *testing.T, howMany int) {
	t.Helper()
	for i := range howMany {
		select {
		case <-b.entered:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d of %d calls entered the pool", i, howMany)
		}
	}
}

func smallPool() resilience.BulkheadPolicy {
	return resilience.BulkheadPolicy{Pool: 2, Queue: 2, Acquisition: 100 * time.Millisecond}
}

func TestTheQueueTakesTheSizeOfThePoolWhenUndeclared(t *testing.T) {
	fake := clock.NewFake(start)
	bulkhead := resilience.NewBulkhead("payments", resilience.BulkheadPolicy{Pool: 2}, fake, nil)

	wall := newBarrier(8)
	decorated := bulkhead.Decorate()(wall.hold)

	// Two fill the pool and two more fill the queue: four are admitted.
	for range 4 {
		go func() { _ = decorated(context.Background(), remoteOp(time.Hour), nil) }()
	}
	wall.awaitEntered(t, 2)
	awaitAlarm(t, fake, 2)

	// The fifth exceeds pool plus queue and is refused at once.
	err := decorated(context.Background(), remoteOp(time.Hour), nil)
	if !errors.Is(err, resilience.ErrBulkheadSaturated) {
		t.Fatalf("the fifth call = %v, want ErrBulkheadSaturated with a pool of 2 and a queue of 2", err)
	}

	close(wall.release)
}

func TestACallBeyondPoolAndQueueIsRefusedWithoutWaiting(t *testing.T) {
	fake := clock.NewFake(start)
	instruments, read := meter(t)
	bulkhead := resilience.NewBulkhead("payments", smallPool(), fake, instruments)

	wall := newBarrier(8)
	decorated := bulkhead.Decorate()(wall.hold)
	for range 4 {
		go func() { _ = decorated(context.Background(), remoteOp(time.Hour), nil) }()
	}
	wall.awaitEntered(t, 2)
	awaitAlarm(t, fake, 2)

	pendingBefore := fake.Pending()
	err := decorated(context.Background(), remoteOp(time.Hour), nil)

	if !errors.Is(err, resilience.ErrBulkheadSaturated) {
		t.Fatalf("decorated() = %v, want ErrBulkheadSaturated", err)
	}
	if got := fake.Pending(); got != pendingBefore {
		t.Fatalf("the refused call armed a timer (%d, was %d): saturation is a refusal, never a wait (RES-14)", got, pendingBefore)
	}
	if got := read(metrics.BulkheadRejectionsTotal); got != 1 {
		t.Errorf("%s = %d, want 1 (RES-23)", metrics.BulkheadRejectionsTotal, got)
	}

	close(wall.release)
}

func TestAQueuedCallGivesUpAtTheAcquisitionWindow(t *testing.T) {
	fake := clock.NewFake(start)
	instruments, read := meter(t)
	bulkhead := resilience.NewBulkhead("payments", smallPool(), fake, instruments)

	wall := newBarrier(8)
	decorated := bulkhead.Decorate()(wall.hold)
	for range 2 {
		go func() { _ = decorated(context.Background(), remoteOp(time.Hour), nil) }()
	}
	wall.awaitEntered(t, 2)

	queued := make(chan error, 1)
	go func() { queued <- decorated(context.Background(), remoteOp(time.Hour), nil) }()
	awaitAlarm(t, fake, 1)

	fake.Advance(smallPool().Acquisition)

	select {
	case err := <-queued:
		if !errors.Is(err, resilience.ErrBulkheadSaturated) {
			t.Fatalf("the queued call = %v, want ErrBulkheadSaturated after the window", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the queued call kept waiting past the acquisition window")
	}
	if got := read(metrics.BulkheadRejectionsTotal); got != 1 {
		t.Errorf("%s = %d, want 1", metrics.BulkheadRejectionsTotal, got)
	}

	close(wall.release)
}

func TestAQueuedCallProceedsWhenASlotFrees(t *testing.T) {
	fake := clock.NewFake(start)
	bulkhead := resilience.NewBulkhead("payments", smallPool(), fake, nil)

	wall := newBarrier(8)
	decorated := bulkhead.Decorate()(wall.hold)
	for range 2 {
		go func() { _ = decorated(context.Background(), remoteOp(time.Hour), nil) }()
	}
	wall.awaitEntered(t, 2)

	queued := make(chan error, 1)
	go func() { queued <- decorated(context.Background(), remoteOp(time.Hour), nil) }()
	awaitAlarm(t, fake, 1)

	close(wall.release)

	select {
	case err := <-queued:
		if err != nil {
			t.Fatalf("the queued call = %v, want nil once a slot freed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the queued call never proceeded after the pool drained")
	}
}

func TestAQueuedCallHonoursTheCallersCancellation(t *testing.T) {
	fake := clock.NewFake(start)
	bulkhead := resilience.NewBulkhead("payments", smallPool(), fake, nil)

	wall := newBarrier(8)
	decorated := bulkhead.Decorate()(wall.hold)
	for range 2 {
		go func() { _ = decorated(context.Background(), remoteOp(time.Hour), nil) }()
	}
	wall.awaitEntered(t, 2)

	caller, cancel := context.WithCancel(context.Background())
	queued := make(chan error, 1)
	go func() { queued <- decorated(caller, remoteOp(time.Hour), nil) }()
	awaitAlarm(t, fake, 1)

	cancel()

	select {
	case err := <-queued:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the queued call = %v, want Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the queued call ignored the cancellation")
	}

	close(wall.release)
}

func TestAReleasedCallGivesItsSlotBack(t *testing.T) {
	fake := clock.NewFake(start)
	bulkhead := resilience.NewBulkhead("payments", smallPool(), fake, nil)

	decorated := bulkhead.Decorate()(resilience.Direct)
	work := func(context.Context) error { return nil }

	// Far more calls than the pool holds, run one after the other: if a slot
	// were leaked, the pool would fill up and the run would stall.
	for range 50 {
		if err := decorated(context.Background(), remoteOp(time.Hour), work); err != nil {
			t.Fatalf("decorated() = %v, want nil — a leaked slot would saturate the pool", err)
		}
	}
}

func TestTheRefusalIsRecordedOnTheSpan(t *testing.T) {
	fake := clock.NewFake(start)
	bulkhead := resilience.NewBulkhead("payments", resilience.BulkheadPolicy{Pool: 1, Queue: 0, Acquisition: 100 * time.Millisecond}, fake, nil)

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})

	wall := newBarrier(4)
	decorated := bulkhead.Decorate()(wall.hold)
	// Pool of 1 and a queue that takes the size of the pool: two are admitted.
	for range 2 {
		go func() { _ = decorated(context.Background(), remoteOp(time.Hour), nil) }()
	}
	wall.awaitEntered(t, 1)
	awaitAlarm(t, fake, 1)

	ctx, span := provider.Tracer("dmpf-observability").Start(context.Background(), "under.test")
	err := decorated(ctx, remoteOp(time.Hour), nil)
	span.End()

	if !errors.Is(err, resilience.ErrBulkheadSaturated) {
		t.Fatalf("decorated() = %v, want ErrBulkheadSaturated", err)
	}

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	found := false
	for _, event := range ended[0].Events() {
		if event.Name == tracing.EventBulkheadSaturated {
			found = true
		}
	}
	if !found {
		t.Fatalf("the span carries no %q event (RES-23)", tracing.EventBulkheadSaturated)
	}

	close(wall.release)
}

func TestAnUndeclaredPoolTakesThePlatformDefault(t *testing.T) {
	fake := clock.NewFake(start)
	bulkhead := resilience.NewBulkhead("payments", resilience.BulkheadPolicy{}, fake, nil)

	decorated := bulkhead.Decorate()(resilience.Direct)

	if err := decorated(context.Background(), remoteOp(time.Hour), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("decorated() = %v, want nil with an undeclared pool", err)
	}
}
