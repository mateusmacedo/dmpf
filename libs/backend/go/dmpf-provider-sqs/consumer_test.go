package dmpfsqs_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfsqs "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-sqs"
)

// handled is one call the scripted sink received.
type handled struct {
	attempt int
	raw     []byte
	ctx     context.Context
}

type scriptedSink struct {
	mu     sync.Mutex
	calls  []handled
	decide func(ctx context.Context, h handled, ack dmpfports.Acknowledger) error
	seen   chan handled
}

func newSink(decide func(ctx context.Context, h handled, ack dmpfports.Acknowledger) error) *scriptedSink {
	return &scriptedSink{decide: decide, seen: make(chan handled, 64)}
}

func (s *scriptedSink) Handle(ctx context.Context, raw []byte, attempt int, ack dmpfports.Acknowledger) error {
	h := handled{attempt: attempt, raw: raw, ctx: ctx}
	s.mu.Lock()
	s.calls = append(s.calls, h)
	s.mu.Unlock()
	s.seen <- h
	return s.decide(ctx, h, ack)
}

func (s *scriptedSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func message(receipt, body string, receiveCount string) sqstypes.Message {
	msg := sqstypes.Message{ReceiptHandle: aws.String(receipt), Body: aws.String(body), MessageId: aws.String("id-" + receipt)}
	if receiveCount != "" {
		msg.Attributes = map[string]string{string(sqstypes.MessageSystemAttributeNameApproximateReceiveCount): receiveCount}
	}
	return msg
}

func newConsumer(c *clock.Fake, sink dmpfsqs.Sink) *dmpfsqs.Consumer {
	cfg := validConfig()
	cfg.Clock = c
	cfg.Rand = func() float64 { return 1 } // full interval: the backoff is deterministic and non-zero
	return &dmpfsqs.Consumer{
		Config:             cfg,
		Channel:            fifoChannel(),
		Sink:               sink,
		Backoff:            retry.Backoff{Base: 2 * time.Second, Factor: 2, Cap: 30 * time.Second},
		VisibilityBase:     30 * time.Second,
		HeartbeatEvery:     10 * time.Second,
		ProcessingDeadline: time.Hour,
		Concurrency:        2,
		WaitTime:           MaxWaitTimeForTests,
	}
}

const MaxWaitTimeForTests = 20 * time.Second

// run starts the consumer over the fake and returns a stop that cancels it and
// waits for Run to return.
func run(t *testing.T, c *dmpfsqs.Consumer, api *dmpfsqs.FakeSQS) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx, api) }()
	return func() {
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
}

func awaitCalls(t *testing.T, api *dmpfsqs.FakeSQS, op string, n int) []dmpfsqs.Call {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for len(api.Calls(op)) < n {
		if time.Now().After(deadline) {
			t.Fatalf("%s calls = %d, want %d", op, len(api.Calls(op)), n)
		}
		time.Sleep(time.Millisecond)
	}
	return api.Calls(op)
}

func awaitAlarm(t *testing.T, c *clock.Fake, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for c.Pending() < n {
		if time.Now().After(deadline) {
			t.Fatalf("pending alarms = %d, want %d", c.Pending(), n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestConsumerValidate(t *testing.T) {
	cases := map[string]struct {
		mutate func(*dmpfsqs.Consumer)
		want   error
	}{
		"no sink":                           {func(c *dmpfsqs.Consumer) { c.Sink = nil }, dmpfsqs.ErrIncompleteConsumer},
		"no concurrency":                    {func(c *dmpfsqs.Consumer) { c.Concurrency = 0 }, dmpfsqs.ErrIncompleteConsumer},
		"heartbeat not before visibility":   {func(c *dmpfsqs.Consumer) { c.HeartbeatEvery = c.VisibilityBase }, dmpfsqs.ErrIncompleteConsumer},
		"heartbeat not twice in visibility": {func(c *dmpfsqs.Consumer) { c.HeartbeatEvery = 16 * time.Second }, dmpfsqs.ErrIncompleteConsumer},
		"short polling":                     {func(c *dmpfsqs.Consumer) { c.WaitTime = 0 }, dmpfsqs.ErrIncompleteConsumer},
		"wait over the api ceiling":         {func(c *dmpfsqs.Consumer) { c.WaitTime = 21 * time.Second }, dmpfsqs.ErrIncompleteConsumer},
		"deadline at the ceiling":           {func(c *dmpfsqs.Consumer) { c.ProcessingDeadline = 12 * time.Hour }, dmpfsqs.ErrDeadlineOverVisibility},
		"inline retry (SQS-11b)":            {func(c *dmpfsqs.Consumer) { c.MaxInlineAttempts = 1 }, dmpfsqs.ErrInlineRetryOnSQS},
		"sns channel without queue":         {func(c *dmpfsqs.Consumer) { c.Channel = snsChannel() }, dmpfsqs.ErrIncompleteConsumer},
		"kafka channel":                     {func(c *dmpfsqs.Consumer) { c.Channel = kafkaChannel() }, dmpfsqs.ErrNotSQSChannel},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := newConsumer(clock.NewFake(start), newSink(nil))
			tc.mutate(c)
			if err := c.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
	t.Run("complete", func(t *testing.T) {
		if err := newConsumer(clock.NewFake(start), newSink(nil)).Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})
	t.Run("sns channel with its queue", func(t *testing.T) {
		c := newConsumer(clock.NewFake(start), newSink(nil))
		c.Channel, c.QueueURL = snsChannel(), standardURL
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})
}

func TestHeartbeatExtendsTheVisibilityWhileTheSinkWorksAndStopsBeforeAck(t *testing.T) {
	c := clock.NewFake(start)
	api := dmpfsqs.NewFakeSQS()
	release := make(chan struct{})
	sink := newSink(func(ctx context.Context, _ handled, ack dmpfports.Acknowledger) error {
		<-release
		return ack.Ack(ctx)
	})
	consumer := newConsumer(c, sink)
	stop := run(t, consumer, api)
	defer stop()

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-1", dmpfsqs.EncodeBody(raw), "1")}})
	<-sink.seen

	for i := 1; i <= 3; i++ {
		// Two alarms: the attempt deadline and the heartbeat timer.
		awaitAlarm(t, c, 2)
		c.Advance(10 * time.Second)
		calls := awaitCalls(t, api, "ChangeMessageVisibility", i)
		in := calls[i-1].Input.(*sqs.ChangeMessageVisibilityInput)
		if *in.ReceiptHandle != "rh-1" || in.VisibilityTimeout != 30 {
			t.Fatalf("tick %d = %+v, want rh-1 and 30s (SQS-08)", i, in)
		}
	}

	close(release)
	deletes := awaitCalls(t, api, "DeleteMessage", 1)
	if *deletes[0].Input.(*sqs.DeleteMessageInput).ReceiptHandle != "rh-1" {
		t.Fatal("Ack deleted another receipt")
	}
	c.Advance(time.Minute)
	time.Sleep(20 * time.Millisecond)
	if len(api.Calls("ChangeMessageVisibility")) != 3 {
		t.Fatalf("visibility extended after the delete: %d ticks (SQS-08)", len(api.Calls("ChangeMessageVisibility")))
	}
}

func TestTwelveHoursFromReceiptCancelTheSinkContext(t *testing.T) {
	c := clock.NewFake(start)
	api := dmpfsqs.NewFakeSQS()
	observed := make(chan error, 1)
	sink := newSink(func(ctx context.Context, _ handled, _ dmpfports.Acknowledger) error {
		<-ctx.Done()
		observed <- ctx.Err()
		return ctx.Err()
	})
	consumer := newConsumer(c, sink)
	consumer.ProcessingDeadline = 11*time.Hour + 59*time.Minute
	stop := run(t, consumer, api)
	defer stop()

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-1", dmpfsqs.EncodeBody(raw), "1")}})
	<-sink.seen

	// The heartbeat keeps the message invisible for hours; the attempt deadline,
	// which Validate keeps below the 12h ceiling, is what ends the attempt.
	awaitAlarm(t, c, 2)
	var err error
	for cancelled := false; !cancelled; {
		c.Advance(10 * time.Second)
		select {
		case err = <-observed:
			cancelled = true
		default:
			if c.Now().After(start.Add(12 * time.Hour)) {
				t.Fatal("the sink context survived past the 12h ceiling (SQS-08b)")
			}
			time.Sleep(200 * time.Microsecond)
		}
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("sink observed %v, want a cancellation", err)
	}
	if ticks := len(api.Calls("ChangeMessageVisibility")); ticks < 4000 {
		t.Fatalf("only %d extensions over ~12h of processing, want the heartbeat to have kept beating (SQS-08)", ticks)
	}
	if len(api.Calls("DeleteMessage")) != 0 {
		t.Fatal("a cancelled attempt deleted the message")
	}
}

func TestReleaseShortensVisibilityToTheBackoffAndNeverDeletes(t *testing.T) {
	c := clock.NewFake(start)
	api := dmpfsqs.NewFakeSQS()
	sink := newSink(func(ctx context.Context, _ handled, ack dmpfports.Acknowledger) error { return ack.Release(ctx) })
	consumer := newConsumer(c, sink)
	stop := run(t, consumer, api)
	defer stop()

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-2", dmpfsqs.EncodeBody(raw), "3")}})
	h := <-sink.seen
	if h.attempt != 3 {
		t.Fatalf("attempt = %d, want the receive count 3 (TRP-52)", h.attempt)
	}
	changes := awaitCalls(t, api, "ChangeMessageVisibility", 1)
	in := changes[0].Input.(*sqs.ChangeMessageVisibilityInput)
	// Backoff base 2s, factor 2, attempt 3 → interval 16s, drawn whole with rand 1.
	if *in.ReceiptHandle != "rh-2" || in.VisibilityTimeout != 16 {
		t.Fatalf("Release = %+v, want rh-2 with the 16s backoff (SQS-10)", in)
	}
	if len(api.Calls("DeleteMessage")) != 0 {
		t.Fatal("Release deleted the message")
	}
}

func TestATickInFlightNeverOverwritesTheRelease(t *testing.T) {
	c := clock.NewFake(start)
	api := dmpfsqs.NewFakeSQS()
	gate := make(chan struct{})
	api.ChangeGate = gate
	releaseNow := make(chan struct{})
	sink := newSink(func(ctx context.Context, _ handled, ack dmpfports.Acknowledger) error {
		<-releaseNow
		return ack.Release(ctx)
	})
	consumer := newConsumer(c, sink)
	stop := run(t, consumer, api)
	defer stop()

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-3", dmpfsqs.EncodeBody(raw), "1")}})
	<-sink.seen

	// A heartbeat tick starts and blocks inside ChangeMessageVisibility...
	awaitAlarm(t, c, 2)
	c.Advance(10 * time.Second)
	awaitCalls(t, api, "ChangeMessageVisibility", 1)
	// ...while the sink releases. Release must wait for the tick.
	close(releaseNow)
	time.Sleep(20 * time.Millisecond)
	if len(api.Calls("ChangeMessageVisibility")) != 1 {
		t.Fatal("Release ran before the tick in flight finished")
	}
	close(gate)
	changes := awaitCalls(t, api, "ChangeMessageVisibility", 2)
	last := changes[1].Input.(*sqs.ChangeMessageVisibilityInput)
	if last.VisibilityTimeout != 4 {
		t.Fatalf("last visibility = %ds, want the 4s backoff (attempt 1) to be the final word", last.VisibilityTimeout)
	}
}

func TestAnUndecodableBodyReachesTheSinkRaw(t *testing.T) {
	c := clock.NewFake(start)
	api := dmpfsqs.NewFakeSQS()
	sink := newSink(func(ctx context.Context, _ handled, ack dmpfports.Acknowledger) error { return ack.Ack(ctx) })
	consumer := newConsumer(c, sink)
	stop := run(t, consumer, api)
	defer stop()

	wrapper := `{"Type":"Notification","TopicArn":"arn:aws:sns:us-east-1:000000000000:t","Message":"x"}`
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-4", wrapper, "1")}})
	h := <-sink.seen
	if string(h.raw) != wrapper {
		t.Fatalf("sink got %q, want the body as transported so the adapter quarantines it", h.raw)
	}
	awaitCalls(t, api, "DeleteMessage", 1)
}

func TestConcurrencyIsRespected(t *testing.T) {
	c := clock.NewFake(start)
	api := dmpfsqs.NewFakeSQS()
	release := make(chan struct{})
	var inFlight, peak int64
	var mu sync.Mutex
	sink := newSink(func(ctx context.Context, _ handled, ack dmpfports.Acknowledger) error {
		mu.Lock()
		inFlight++
		peak = max(peak, inFlight)
		mu.Unlock()
		<-release
		mu.Lock()
		inFlight--
		mu.Unlock()
		return ack.Ack(ctx)
	})
	consumer := newConsumer(c, sink)
	consumer.Concurrency = 2
	stop := run(t, consumer, api)
	defer stop()

	raw, _ := validRaw(t, "k1")
	var msgs []sqstypes.Message
	for _, r := range []string{"a", "b", "c", "d"} {
		msgs = append(msgs, message(r, dmpfsqs.EncodeBody(raw), "1"))
	}
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: msgs})
	<-sink.seen
	<-sink.seen
	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	if peak != 2 || sink.count() != 2 {
		mu.Unlock()
		t.Fatalf("in flight = %d, handled = %d; want exactly the concurrency of 2", peak, sink.count())
	}
	mu.Unlock()
	close(release)
	awaitCalls(t, api, "DeleteMessage", 4)
}

func TestAFailingReceiveBacksOffInsteadOfSpinning(t *testing.T) {
	c := clock.NewFake(start)
	api := dmpfsqs.NewFakeSQS()
	api.ReceiveErr = errors.New("access denied")
	consumer := newConsumer(c, newSink(nil))
	stop := run(t, consumer, api)
	defer stop()

	awaitCalls(t, api, "ReceiveMessage", 1)
	awaitAlarm(t, c, 1)
	time.Sleep(20 * time.Millisecond)
	if n := len(api.Calls("ReceiveMessage")); n != 1 {
		t.Fatalf("receives = %d while the backoff runs, want 1", n)
	}
	// Backoff base 2s, factor 2, first failure → 4s, drawn whole with rand 1.
	c.Advance(4 * time.Second)
	awaitCalls(t, api, "ReceiveMessage", 2)
}

func TestASinkPanicLeavesTheMessageToItsVisibilityAndKeepsConsuming(t *testing.T) {
	c := clock.NewFake(start)
	api := dmpfsqs.NewFakeSQS()
	sink := newSink(func(ctx context.Context, h handled, ack dmpfports.Acknowledger) error {
		if h.attempt == 1 {
			panic("bad envelope")
		}
		return ack.Ack(ctx)
	})
	consumer := newConsumer(c, sink)
	stop := run(t, consumer, api)
	defer stop()

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-p", dmpfsqs.EncodeBody(raw), "1")}})
	<-sink.seen
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-q", dmpfsqs.EncodeBody(raw), "2")}})
	<-sink.seen
	deletes := awaitCalls(t, api, "DeleteMessage", 1)
	if got := *deletes[0].Input.(*sqs.DeleteMessageInput).ReceiptHandle; got != "rh-q" {
		t.Fatalf("deleted %q, want rh-q: the panicked receipt keeps its visibility, the next one is consumed", got)
	}
}
