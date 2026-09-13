// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (SQS-07..13, TRP-26, TRP-52) ou FND-08 (MET-11) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfsqs

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel/metric"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/retry"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/attempt"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/channel"
)

// Sink is what the consumer hands each message to: the consumer adapter of the
// app block, which decodes, runs the application service and records its
// gesture on the acknowledger after the local commit (TRP-26).
type Sink interface {
	Handle(ctx context.Context, raw []byte, attempt int, ack dmpfports.Acknowledger) error
}

// MaxVisibility is the ceiling on accumulated extensions from receipt (SQS-08b).
const MaxVisibility = channel.MaxVisibility

// MaxWaitTime is the API's ceiling on WaitTimeSeconds; below one second the
// receive is short polling, which turns the loop into a busy one.
const MaxWaitTime = 20 * time.Second

// Consumer receives one channel's queue with a pool of workers: the receive
// count is the attempt (TRP-52), a heartbeat keeps the message invisible
// (SQS-08), one gesture ends it (SQS-09, SQS-10); no inline retry (SQS-11b).
type Consumer struct {
	Config             Config
	Channel            channel.Channel
	QueueURL           string
	Sink               Sink
	Backoff            retry.Backoff
	VisibilityBase     time.Duration
	HeartbeatEvery     time.Duration
	ProcessingDeadline time.Duration
	Concurrency        int
	MaxInlineAttempts  int
	WaitTime           time.Duration

	busy  atomic.Int64
	depth atomic.Int64
}

// Validate refuses a consumer the queue could not run safely: no sink or queue,
// a heartbeat that does not fit twice in the visibility, short polling, a
// deadline at or over the 12h ceiling (SQS-08b, SQS-13), or inline retry (SQS-11b).
func (c *Consumer) Validate() error {
	if err := c.Config.Validate(); err != nil {
		return err
	}
	if err := c.Channel.Validate(); err != nil {
		return err
	}
	switch {
	case c.Channel.Transport != channel.SQS && c.Channel.Transport != channel.SNSSQS:
		return fmt.Errorf("%w: %s", ErrNotSQSChannel, c.Channel.Name)
	case c.Sink == nil:
		return fmt.Errorf("%w: sink", ErrIncompleteConsumer)
	case c.queueURL() == "":
		return fmt.Errorf("%w: queue url (an sns-sqs channel names the topic; the consumer needs its queue)", ErrIncompleteConsumer)
	case c.Concurrency <= 0:
		return fmt.Errorf("%w: concurrency", ErrIncompleteConsumer)
	case c.VisibilityBase <= 0 || c.VisibilityBase > MaxVisibility:
		return fmt.Errorf("%w: visibility base must be in (0, 12h]", ErrIncompleteConsumer)
	case c.HeartbeatEvery <= 0 || 2*c.HeartbeatEvery > c.VisibilityBase:
		// A tick may take one whole interval before the next is armed, so
		// two intervals must fit in the visibility for it to be renewed in time.
		return fmt.Errorf("%w: heartbeat %v must fit twice in the visibility %v (SQS-08)", ErrIncompleteConsumer, c.HeartbeatEvery, c.VisibilityBase)
	case c.WaitTime < time.Second || c.WaitTime > MaxWaitTime:
		return fmt.Errorf("%w: wait time %v must be in [1s, 20s]: long polling, within the API ceiling", ErrIncompleteConsumer, c.WaitTime)
	case c.ProcessingDeadline <= 0:
		return fmt.Errorf("%w: processing deadline", ErrIncompleteConsumer)
	case c.ProcessingDeadline >= MaxVisibility:
		// SQS-13 asks the deadline to fit the visibility applied; with the
		// heartbeat renewing it, the ceiling is what bounds the applied visibility.
		return fmt.Errorf("%w: %v is not below the 12h visibility ceiling (SQS-08b, SQS-13)", ErrDeadlineOverVisibility, c.ProcessingDeadline)
	case c.MaxInlineAttempts != 0:
		return fmt.Errorf("%w: MaxInlineAttempts = %d", ErrInlineRetryOnSQS, c.MaxInlineAttempts)
	case c.Backoff.Base <= 0:
		return fmt.Errorf("%w: backoff base", ErrIncompleteConsumer)
	}
	return nil
}

func (c *Consumer) queueURL() string {
	if c.QueueURL != "" {
		return c.QueueURL
	}
	if c.Channel.Transport == channel.SQS {
		return c.Channel.Address
	}
	return ""
}

// Run receives until ctx is done. Slots are taken before each receive and the
// request asks for no more than the free ones, so every message received has a
// worker at once: nothing waits invisible with its heartbeat unstarted (SQS-08).
func (c *Consumer) Run(ctx context.Context, api sqsAPI) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if api == nil {
		return fmt.Errorf("%w: sqs client", ErrIncompleteConfig)
	}

	slots := make(chan struct{}, c.Concurrency)
	var wg sync.WaitGroup
	defer wg.Wait()
	random := c.Config.Rand
	if random == nil {
		random = rand.Float64
	}
	sleep := clock.NewSleeper(c.Config.Clock)
	failures := 0

	in := &sqs.ReceiveMessageInput{
		QueueUrl:                    aws.String(c.queueURL()),
		WaitTimeSeconds:             visibilitySeconds(c.WaitTime),
		VisibilityTimeout:           visibilitySeconds(c.VisibilityBase),
		MessageSystemAttributeNames: []sqstypes.MessageSystemAttributeName{sqstypes.MessageSystemAttributeNameApproximateReceiveCount},
	}
	for {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		taken := 1
		for taken < min(c.Concurrency, 10) {
			select {
			case slots <- struct{}{}:
				taken++
				continue
			default:
			}
			break
		}
		in.MaxNumberOfMessages = int32(taken)

		out, err := api.ReceiveMessage(ctx, in)
		receivedAt := c.Config.Clock.Now()
		if ctx.Err() != nil {
			c.release(slots, taken)
			return ctx.Err()
		}
		if err != nil {
			c.release(slots, taken)
			failures++
			c.Config.logger().WarnContext(ctx, "dmpfsqs: receive failed", slog.Int("consecutive_failures", failures), slog.String("error_category", categoryOf(err)))
			// A queue that cannot be read must not be hammered at the RTT.
			if err := sleep(ctx, c.Backoff.Next(failures, random)); err != nil {
				return ctx.Err()
			}
			continue
		}
		failures = 0
		c.depth.Store(int64(len(out.Messages)))
		c.observeSaturation(ctx)
		c.dispatch(ctx, api, slots, &wg, taken, out.Messages, receivedAt)
	}
}

// dispatch starts one worker per message. Should the broker hand over more
// than the slots asked for, the extra messages wait for a slot here rather
// than exceed the concurrency; at shutdown they are left to their visibility.
func (c *Consumer) dispatch(ctx context.Context, api sqsAPI, slots chan struct{}, wg *sync.WaitGroup, taken int, msgs []sqstypes.Message, receivedAt time.Time) {
	started := 0
	defer func() {
		c.depth.Store(0)
		c.release(slots, taken-started)
	}()
	for _, msg := range msgs {
		if started == taken {
			select {
			case slots <- struct{}{}:
				taken++
			case <-ctx.Done():
				return
			}
		}
		started++
		c.depth.Store(int64(len(msgs) - started))
		wg.Add(1)
		c.busy.Add(1)
		go func(msg sqstypes.Message) {
			defer wg.Done()
			defer c.busy.Add(-1)
			defer func() { <-slots }()
			c.process(ctx, api, msg, receivedAt)
		}(msg)
	}
}

func (c *Consumer) release(slots chan struct{}, n int) {
	for range n {
		<-slots
	}
}

// process handles one receipt: the attempt from the receive count (TRP-52), a
// heartbeat until the gesture or the ceiling (SQS-08, SQS-08b), the body decoded
// once (SQS-01) — raw to the sink when it is not the envelope — and the gesture.
func (c *Consumer) process(ctx context.Context, api sqsAPI, msg sqstypes.Message, receivedAt time.Time) {
	clk := c.Config.Clock
	queueURL, receipt := c.queueURL(), aws.ToString(msg.ReceiptHandle)
	attemptNo := receiveCount(msg)

	msgCtx, cancelMsg := context.WithCancel(ctx)
	defer cancelMsg()
	hb := startHeartbeat(msgCtx, clk, c.HeartbeatEvery, receivedAt.Add(MaxVisibility), func(ctx context.Context, remaining time.Duration) error {
		_, err := api.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
			QueueUrl: aws.String(queueURL), ReceiptHandle: aws.String(receipt), VisibilityTimeout: visibilitySeconds(min(c.VisibilityBase, remaining)),
		})
		if err != nil {
			c.Config.logger().WarnContext(ctx, "dmpfsqs: visibility extension failed; attempt cancelled", slog.String("error_category", categoryOf(err)))
		}
		return err
	}, cancelMsg)
	defer hb.Stop()

	random := c.Config.Rand
	if random == nil {
		random = rand.Float64
	}
	ack := newAcknowledger(api, queueURL, receipt, c.Backoff.Next(attemptNo, random), hb.Stop)

	raw, err := DecodeBody(aws.ToString(msg.Body))
	if err != nil {
		// Not the envelope: the sink gets the body as transported and the
		// adapter quarantines what does not decode (INB-10).
		raw = []byte(aws.ToString(msg.Body))
	}

	attemptCtx, cancelAttempt := clk.WithTimeout(msgCtx, c.ProcessingDeadline)
	defer cancelAttempt()
	err = c.handle(attemptCtx, raw, attemptNo, ack)
	switch {
	case err != nil:
		c.Config.logger().WarnContext(ctx, "dmpfsqs: sink failed",
			slog.String("channel", c.Channel.Name), slog.Int("attempt", attemptNo), slog.Bool("gesture_recorded", ack.isDisposed()),
			slog.String("error_category", categoryOf(err)))
	case !ack.isDisposed():
		c.Config.logger().ErrorContext(ctx, "dmpfsqs: sink returned without a gesture; visibility left to expire (SQS-10)",
			slog.String("channel", c.Channel.Name), slog.Int("attempt", attemptNo))
	}
}

// handle runs the sink on one receipt. A panic inside it — the sink decodes
// bytes from outside the process — is a failed attempt without a gesture, not
// the end of every worker of the consumer.
func (c *Consumer) handle(ctx context.Context, raw []byte, attemptNo int, ack *acknowledger) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: %v", ErrSinkPanicked, recovered)
		}
	}()
	return c.Sink.Handle(attempt.WithContext(ctx, attemptNo), raw, attemptNo, ack)
}

func (a *acknowledger) isDisposed() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.disposed
}

// receiveCount is the queue's own count of deliveries; one when it is absent.
func receiveCount(msg sqstypes.Message) int {
	n, err := strconv.Atoi(msg.Attributes[string(sqstypes.MessageSystemAttributeNameApproximateReceiveCount)])
	if err != nil || n <= 0 {
		return 1
	}
	return n
}

// observeSaturation records MET-11 once per receive: the share of busy workers
// and the messages just received, not yet handed to their worker.
func (c *Consumer) observeSaturation(ctx context.Context) {
	if c.Config.Instruments == nil {
		return
	}
	labels := metric.WithAttributes(metrics.Labels{}.Service(c.Config.Service).Attributes()...)
	c.Config.Instruments.PoolUtilization.Record(ctx, float64(c.busy.Load())/float64(c.Concurrency), labels)
	c.Config.Instruments.QueueDepth.Record(ctx, c.depth.Load(), labels)
}
