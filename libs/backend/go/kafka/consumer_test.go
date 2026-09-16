package kafka_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const topic = "sales.order.placed.v1"

// handled is one call the scripted sink received.
type handled struct {
	offset  int64
	attempt int
	value   string
}

// scriptedSink decides per offset: acked, released, or a custom function; it
// records every call so the test reads the order and the attempt numbers.
type scriptedSink struct {
	mu      sync.Mutex
	calls   []handled
	script  map[int64]func(ctx context.Context, attempt int, ack ports.Acknowledger) error
	byValue map[string]int64
}

func newSink() *scriptedSink {
	return &scriptedSink{script: map[int64]func(context.Context, int, ports.Acknowledger) error{}, byValue: map[string]int64{}}
}

func (s *scriptedSink) on(offset int64, fn func(ctx context.Context, attempt int, ack ports.Acknowledger) error) {
	s.script[offset] = fn
	s.byValue[value(offset)] = offset
}

func (s *scriptedSink) Handle(ctx context.Context, raw []byte, attempt int, ack ports.Acknowledger) error {
	offset := s.byValue[string(raw)]
	s.mu.Lock()
	s.calls = append(s.calls, handled{offset: offset, attempt: attempt, value: string(raw)})
	s.mu.Unlock()
	if fn, ok := s.script[offset]; ok {
		return fn(ctx, attempt, ack)
	}
	return ack.Ack(ctx)
}

func (s *scriptedSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *scriptedSink) snapshot() []handled {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]handled(nil), s.calls...)
}

func value(offset int64) string { return "record-" + string(rune('a'+offset)) }

func recordsAt(offsets ...int64) []*kgo.Record {
	out := make([]*kgo.Record, 0, len(offsets))
	for _, o := range offsets {
		out = append(out, &kgo.Record{Offset: o, Value: []byte(value(o))})
	}
	return out
}

func alwaysAck(ctx context.Context, _ int, ack ports.Acknowledger) error { return ack.Ack(ctx) }

func alwaysRelease(ctx context.Context, _ int, ack ports.Acknowledger) error {
	return ack.Release(ctx)
}

func newConsumer(sink kafka.Sink) *kafka.Consumer {
	cfg := publishConfig()
	cfg.Clock = clock.System()
	cfg.Rand = func() float64 { return 0 }
	return &kafka.Consumer{
		Config:             cfg,
		Channel:            ordersChannel(),
		Sink:               sink,
		Backoff:            retry.Backoff{Base: time.Millisecond, Factor: 2, Cap: 10 * time.Millisecond},
		ProcessingDeadline: 2 * time.Second,
		RebalanceTimeout:   10 * time.Second,
		QueuePerPartition:  16,
	}
}

// consume runs the consumer over the fake until the condition holds or the
// deadline passes, then stops it and returns its final error.
func consume(t *testing.T, c *kafka.Consumer, fake *kafka.FakeClient, until func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.RunWith(ctx, fake) }()

	deadline := time.Now().Add(5 * time.Second)
	for !until() {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("the condition did not hold within 5s")
		}
		time.Sleep(2 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return after cancellation")
	}
}

func commits(fake *kafka.FakeClient) []int64 {
	committed := fake.Commits()
	out := make([]int64, 0, len(committed))
	for _, r := range committed {
		out = append(out, r.Offset)
	}
	return out
}

func TestConsumerValidate(t *testing.T) {
	cases := map[string]func(*kafka.Consumer){
		"no sink":                      func(c *kafka.Consumer) { c.Sink = nil },
		"no attempt limit":             func(c *kafka.Consumer) { c.Channel.Retry.MaxAttempts = 0 },
		"no queue":                     func(c *kafka.Consumer) { c.QueuePerPartition = 0 },
		"deadline not below rebalance": func(c *kafka.Consumer) { c.ProcessingDeadline = c.RebalanceTimeout },
		"no backoff":                   func(c *kafka.Consumer) { c.Backoff = retry.Backoff{} },
		"not kafka":                    func(c *kafka.Consumer) { c.Channel = sqsChannel() },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := newConsumer(newSink())
			mutate(c)
			if err := c.Validate(); err == nil {
				t.Fatal("Validate() = nil, want a refusal")
			}
		})
	}
	if err := newConsumer(newSink()).Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestThreeAcknowledgedRecordsCommitThePrefix(t *testing.T) {
	sink := newSink()
	for _, o := range []int64{0, 1, 2} {
		sink.on(o, alwaysAck)
	}
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1, 2)...)
	c := newConsumer(sink)

	consume(t, c, fake, func() bool { return len(fake.Commits()) >= 3 && c.PendingOf(topic, 0) == 0 })

	if got := commits(fake); len(got) != 3 || got[2] != 2 {
		t.Fatalf("commits = %v, want the records 0, 1, 2 in order (kgo commits offset+1)", got)
	}
}

func TestARecordWithoutGestureStallsThePartitionAndFixesTheCommitCeiling(t *testing.T) {
	sink := newSink()
	sink.on(0, alwaysAck)
	sink.on(1, func(context.Context, int, ports.Acknowledger) error { return nil }) // no gesture
	sink.on(2, alwaysAck)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1, 2)...)
	c := newConsumer(sink)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.RunWith(ctx, fake) }()
	awaitCondition(t, func() bool { return c.StalledAt(topic, 0) })
	if _, paused := fake.Paused()[topic]; !paused {
		t.Fatal("the stalled partition is not paused")
	}
	cancel()
	<-done

	if got := commits(fake); len(got) != 1 || got[0] != 0 {
		t.Fatalf("commits = %v, want only the record 0: 1 is pending and fixes the ceiling (TRP-29)", got)
	}
	if sink.count() != 2 {
		t.Fatalf("handled %d records, want 2: nothing after the pending record is processed (KFK-09)", sink.count())
	}
	if paused := fake.Paused(); len(paused) != 0 {
		t.Fatalf("paused after shutdown = %v, want none", paused)
	}
}

func TestReleaseRedeliversWithTheNextAttempt(t *testing.T) {
	sink := newSink()
	sink.on(0, func(ctx context.Context, attempt int, ack ports.Acknowledger) error {
		if attempt == 1 {
			return ack.Release(ctx)
		}
		return ack.Ack(ctx)
	})
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)
	c := newConsumer(sink)

	consume(t, c, fake, func() bool { return len(fake.Commits()) >= 1 })

	calls := sink.snapshot()
	if len(calls) != 2 || calls[0].attempt != 1 || calls[1].attempt != 2 || calls[0].value != calls[1].value {
		t.Fatalf("calls = %v, want the same value handled twice with attempts 1 and 2 (KFK-10, TRP-52)", calls)
	}
	if _, paused := fake.Paused()[topic]; paused {
		t.Fatal("the partition stayed paused after the backoff")
	}
}

func TestAttemptsExhaustedLeaveTheRecordPendingAndStallThePartition(t *testing.T) {
	sink := newSink()
	sink.on(0, alwaysRelease)
	sink.on(1, alwaysAck)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1)...)
	c := newConsumer(sink) // channel limit is 3

	consume(t, c, fake, func() bool { return c.StalledAt(topic, 0) })

	if sink.count() != 3 {
		t.Fatalf("handled %d times, want exactly the channel's 3 attempts of the first record", sink.count())
	}
	if len(fake.Commits()) != 0 {
		t.Fatalf("commits = %v, want none: an exhausted record is never skipped (TRP-47)", commits(fake))
	}
}

func TestAFaultBetweenQuarantineAndAckDoesNotAdvanceTheOffsetNorRepeatTheQuarantine(t *testing.T) {
	dlqFake := kafka.NewFakeClient()
	dlq, err := kafka.NewDLQWith(publishConfig(), ordersChannel(), dlqFake)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")

	sink := newSink()
	sink.on(0, func(ctx context.Context, _ int, _ ports.Acknowledger) error {
		if err := dlq.Quarantine(ctx, ports.Contained{Consumer: "billing", Reason: ports.ReasonTerminalFailure, Envelope: raw}); err != nil {
			return err
		}
		return errors.New("crashed between quarantine and ack")
	})
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)
	c := newConsumer(sink)

	consume(t, c, fake, func() bool { return c.StalledAt(topic, 0) })

	if n := len(dlqFake.Produced()); n != 1 {
		t.Fatalf("the DLQ received %d records, want exactly 1: an error without gesture is not a Release", n)
	}
	if len(fake.Commits()) != 0 {
		t.Fatalf("commits = %v, want none (TRP-30)", commits(fake))
	}
	headers := map[string]string{}
	for _, h := range dlqFake.Produced()[0].Headers {
		headers[h.Key] = string(h.Value)
	}
	if headers[kafka.HeaderAttempt] != "1" {
		t.Fatalf("dmpf-attempt = %q, want the attempt the consumer recorded in the context (TRP-52)", headers[kafka.HeaderAttempt])
	}
}

func TestRevocationCancelsTheAttemptAndKeepsTheContiguousCommit(t *testing.T) {
	blocked := make(chan struct{})
	observed := make(chan error, 1)
	sink := newSink()
	sink.on(0, alwaysAck)
	sink.on(1, func(ctx context.Context, _ int, _ ports.Acknowledger) error {
		close(blocked)
		<-ctx.Done()
		observed <- ctx.Err()
		return ctx.Err()
	})
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1)...)
	c := newConsumer(sink)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.RunWith(ctx, fake) }()

	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("the second record never reached the sink")
	}
	c.Revoke(context.Background(), map[string][]int32{topic: {0}})

	select {
	case err := <-observed:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the attempt observed %v, want cancellation (TRP-48)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the attempt in flight did not observe the revocation")
	}
	if got := commits(fake); len(got) != 1 || got[0] != 0 {
		t.Fatalf("commits = %v, want only the record acknowledged before the revocation", got)
	}
	if c.Workers() != 0 {
		t.Fatalf("workers = %d after revocation, want 0", c.Workers())
	}
	cancel()
	<-done
}

func TestTwoBatchesOfOnePartitionShareOneWorkerInOrder(t *testing.T) {
	sink := newSink()
	for _, o := range []int64{0, 1, 2, 3} {
		sink.on(o, alwaysAck)
	}
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1)...)
	fake.Feed(topic, 0, recordsAt(2, 3)...)
	fake.Feed(topic, 1, recordsAt(4)...)
	sink.on(4, alwaysAck)
	c := newConsumer(sink)

	consume(t, c, fake, func() bool { return len(fake.Commits()) >= 5 })

	var order []int64
	for _, h := range sink.snapshot() {
		if h.offset != 4 {
			order = append(order, h.offset)
		}
	}
	for i := range order {
		if order[i] != int64(i) {
			t.Fatalf("partition 0 handled in order %v, want 0,1,2,3 (KFK-09)", order)
		}
	}
	if fake.Rebalances() < 3 {
		t.Fatalf("AllowRebalance called %d times, want one per poll", fake.Rebalances())
	}
}

func TestRevocationLiftsTheFetchPauseOfAStalledPartition(t *testing.T) {
	sink := newSink()
	sink.on(0, func(context.Context, int, ports.Acknowledger) error { return nil }) // no gesture
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1)...)
	c := newConsumer(sink)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.RunWith(ctx, fake) }()
	awaitCondition(t, func() bool { return c.StalledAt(topic, 0) })
	if _, paused := fake.Paused()[topic]; !paused {
		t.Fatal("the stalled partition is not paused")
	}

	c.Revoke(ctx, map[string][]int32{topic: {0}})
	if paused := fake.Paused(); len(paused) != 0 {
		t.Fatalf("paused after revocation = %v, want none: kgo keeps the pause across rebalances", paused)
	}
	cancel()
	<-done
}

func TestRevocationDoesNotDrainTheQueueThroughTheSink(t *testing.T) {
	sink := newSink()
	started := make(chan struct{}, 16)
	slow := func(ctx context.Context, _ int, ack ports.Acknowledger) error {
		started <- struct{}{}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return ack.Ack(ctx)
		}
	}
	for o := int64(0); o < 10; o++ {
		sink.on(o, slow)
	}
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)...)
	c := newConsumer(sink)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.RunWith(ctx, fake) }()
	<-started

	before := time.Now()
	c.Revoke(ctx, map[string][]int32{topic: {0}})
	if took := time.Since(before); took > 40*time.Millisecond {
		t.Fatalf("revocation took %v, want the cancelled attempt to return at once", took)
	}
	time.Sleep(20 * time.Millisecond)
	if n := sink.count(); n != 1 {
		t.Fatalf("the sink ran %d times after revocation, want 1: the queue is not drained through it", n)
	}
	cancel()
	<-done
}

func TestAnAssignmentBeforeRunHasAClientAndAMap(t *testing.T) {
	c := newConsumer(newSink())
	fake := kafka.NewFakeClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.Assign(ctx, fake, map[string][]int32{topic: {0, 1}})
	if c.Workers() != 2 {
		t.Fatalf("workers = %d, want 2 from the assignment alone", c.Workers())
	}
	c.Revoke(ctx, map[string][]int32{topic: {0, 1}})
	if c.Workers() != 0 {
		t.Fatalf("workers = %d after revocation, want 0", c.Workers())
	}
}

func TestASinkPanicIsAFailedAttemptNotACrash(t *testing.T) {
	sink := newSink()
	sink.on(0, func(context.Context, int, ports.Acknowledger) error { panic("bad envelope") })
	sink.on(1, alwaysAck)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1)...)
	c := newConsumer(sink)

	consume(t, c, fake, func() bool { return c.StalledAt(topic, 0) })

	if sink.count() != 1 {
		t.Fatalf("handled %d records, want 1: the panic stalls the partition like any attempt without a gesture", sink.count())
	}
	if len(fake.Commits()) != 0 {
		t.Fatalf("commits = %v, want none", commits(fake))
	}
}

func awaitCondition(t *testing.T, until func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !until() {
		if time.Now().After(deadline) {
			t.Fatal("the condition did not hold within 5s")
		}
		time.Sleep(2 * time.Millisecond)
	}
}
