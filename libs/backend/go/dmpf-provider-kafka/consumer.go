// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (KFK-07..12, KFK-19, TRP-26..29, TRP-47, TRP-48) ou FND-08 (MET-11) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfkafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/metric"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/retry"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/channel"
)

// Sink is what the consumer hands each record to: the consumer adapter of the
// app block, which decodes, runs the application service and records its
// gesture on the acknowledger after the local commit (TRP-26).
type Sink interface {
	Handle(ctx context.Context, raw []byte, attempt int, ack dmpfports.Acknowledger) error
}

// Consumer consumes one catalogued channel with one persistent worker per
// partition (KFK-09), commits only the contiguous prefix (TRP-29), retries
// inline up to the channel's limit (KFK-10, TRP-47) and cancels on revocation.
type Consumer struct {
	Config             Config
	Channel            channel.Channel
	Sink               Sink
	Backoff            retry.Backoff
	ProcessingDeadline time.Duration
	RebalanceTimeout   time.Duration
	QueuePerPartition  int
	MaxPollRecords     int

	mu      sync.Mutex
	workers map[key]*partitionWorker
	client  client
}

type key struct {
	topic     string
	partition int32
}

// Validate refuses a consumer the broker could not run safely: no sink, a
// non-Kafka channel, no attempt limit, no queue capacity, or a processing
// deadline the rebalance could not wait for (KFK-19, TRP-48).
func (c *Consumer) Validate() error {
	if err := c.Config.Validate(); err != nil {
		return err
	}
	if err := c.Channel.Validate(); err != nil {
		return err
	}
	switch {
	case c.Channel.Transport != channel.Kafka:
		return fmt.Errorf("%w: %s", ErrNotKafkaChannel, c.Channel.Name)
	case c.Sink == nil:
		return fmt.Errorf("%w: sink", ErrIncompleteConsumer)
	case c.Channel.Retry.MaxAttempts <= 0:
		return fmt.Errorf("%w: attempt limit (TRP-31)", ErrIncompleteConsumer)
	case c.QueuePerPartition <= 0:
		return fmt.Errorf("%w: queue per partition", ErrIncompleteConsumer)
	case c.ProcessingDeadline <= 0:
		return fmt.Errorf("%w: processing deadline", ErrIncompleteConsumer)
	case c.RebalanceTimeout <= 0 || c.ProcessingDeadline >= c.RebalanceTimeout:
		return fmt.Errorf("%w: processing deadline %v must be below the rebalance timeout %v (KFK-19)", ErrIncompleteConsumer, c.ProcessingDeadline, c.RebalanceTimeout)
	case c.Backoff.Base <= 0:
		return fmt.Errorf("%w: backoff base", ErrIncompleteConsumer)
	}
	return nil
}

// Run joins the group and consumes until ctx is done. Auto-commit is off
// (TRP-28); the rebalance is blocked while a poll is being dispatched, so a
// revocation never races the enqueue of records it would then cancel.
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.Validate(); err != nil {
		return err
	}
	// The group manager starts inside NewClient, so the assignment callback
	// may run before run() stores the client: the map must exist by then.
	c.mu.Lock()
	if c.workers == nil {
		c.workers = map[key]*partitionWorker{}
	}
	c.mu.Unlock()
	cl, err := newClient(c.Config,
		kgo.ConsumerGroup(c.Channel.Group),
		kgo.ConsumeTopics(c.Channel.Address),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.RebalanceTimeout(c.RebalanceTimeout),
		kgo.ConsumeResetOffset(c.initialOffset()),
		kgo.OnPartitionsAssigned(c.assigned),
		kgo.OnPartitionsRevoked(c.revoked),
		kgo.OnPartitionsLost(c.revoked),
	)
	if err != nil {
		return fmt.Errorf("dmpfkafka: consumer: %w", err)
	}
	// Close would wait for a rebalance permission the stopped poll loop never
	// grants again; kgo documents CloseAllowingRebalance for BlockRebalanceOnPoll.
	defer cl.CloseAllowingRebalance()
	return c.run(ctx, cl)
}

func (c *Consumer) initialOffset() kgo.Offset {
	if c.Channel.Redelivery.Params["initialOffset"] == "latest" {
		return kgo.NewOffset().AtEnd()
	}
	return kgo.NewOffset().AtStart()
}

func (c *Consumer) run(ctx context.Context, cl client) error {
	if err := c.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	c.client = cl
	if c.workers == nil {
		c.workers = map[key]*partitionWorker{}
	}
	c.mu.Unlock()
	defer c.stopAll(context.WithoutCancel(ctx))

	maxPoll := c.MaxPollRecords
	if maxPoll <= 0 {
		maxPoll = 100
	}
	for {
		fetches := cl.PollRecords(ctx, maxPoll)
		if ctx.Err() != nil {
			cl.AllowRebalance()
			return ctx.Err()
		}
		for _, fetchErr := range fetches.Errors() {
			if errors.Is(fetchErr.Err, context.Canceled) {
				continue
			}
			c.Config.logger().WarnContext(ctx, "dmpfkafka: fetch error",
				slog.String("topic", fetchErr.Topic), slog.Int("partition", int(fetchErr.Partition)),
				slog.String("error_category", categoryOf(fetchErr.Err)))
		}
		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			if len(p.Records) == 0 {
				return
			}
			w := c.worker(ctx, cl, p.Topic, p.Partition)
			for _, record := range p.Records {
				w.enqueue(record)
			}
		})
		c.observeSaturation(ctx)
		cl.AllowRebalance()
	}
}

// worker returns the persistent worker of the partition, creating it on first
// sight — the assignment callback also creates it, but a fake client that
// never calls back must still route two batches of one partition to one worker.
func (c *Consumer) worker(ctx context.Context, cl client, topic string, partition int32) *partitionWorker {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := key{topic: topic, partition: partition}
	if w, found := c.workers[k]; found {
		return w
	}
	w := newPartitionWorker(ctx, c, cl, k)
	c.workers[k] = w
	return w
}

func (c *Consumer) assigned(ctx context.Context, cl *kgo.Client, assigned map[string][]int32) {
	var fallback client
	if cl != nil {
		fallback = cl
	}
	c.assign(ctx, fallback, assigned)
}

// assign creates the worker of every assigned partition. The callback's own
// client is adopted when run() has not stored one yet — kgo may assign before
// NewClient returns.
func (c *Consumer) assign(ctx context.Context, fallback client, assigned map[string][]int32) {
	c.mu.Lock()
	if c.client == nil {
		c.client = fallback
	}
	if c.workers == nil {
		c.workers = map[key]*partitionWorker{}
	}
	cl := c.client
	c.mu.Unlock()
	for topic, partitions := range assigned {
		for _, partition := range partitions {
			c.worker(ctx, cl, topic, partition)
		}
	}
}

// revoked cancels the work of every lost partition, waits for the worker to
// leave — no detached work survives the revocation (TRP-48) —, lifts its fetch
// pause and commits only what was already contiguously acknowledged (TRP-29).
func (c *Consumer) revoked(ctx context.Context, _ *kgo.Client, lost map[string][]int32) {
	for topic, partitions := range lost {
		for _, partition := range partitions {
			c.mu.Lock()
			k := key{topic: topic, partition: partition}
			w, found := c.workers[k]
			delete(c.workers, k)
			cl := c.client
			c.mu.Unlock()
			if !found {
				continue
			}
			w.markRevoked()
			w.stop()
			w.resumeFetch()
			w.commitContiguous(ctx, cl, true)
		}
	}
}

// stopAll drains every worker at shutdown and commits what was acknowledged,
// under a deadline of one rebalance timeout: an unresponsive broker must not
// hold the process open forever.
func (c *Consumer) stopAll(ctx context.Context) {
	c.mu.Lock()
	workers := c.workers
	c.workers = map[key]*partitionWorker{}
	cl := c.client
	c.mu.Unlock()
	drain, cancel := context.WithTimeout(ctx, c.RebalanceTimeout)
	defer cancel()
	for _, w := range workers {
		w.markRevoked()
		w.stop()
		w.resumeFetch()
		w.commitContiguous(drain, cl, true)
	}
}

// observeSaturation records MET-11 once per poll: the share of assigned
// partitions whose worker is busy, and the records waiting in every queue.
func (c *Consumer) observeSaturation(ctx context.Context) {
	if c.Config.Instruments == nil {
		return
	}
	c.mu.Lock()
	busy, depth := 0, 0
	total := len(c.workers)
	for _, w := range c.workers {
		if w.busy.Load() {
			busy++
		}
		depth += w.Queued()
	}
	c.mu.Unlock()
	if total == 0 {
		return
	}
	labels := metric.WithAttributes(metrics.Labels{}.Service(c.Config.Service).Attributes()...)
	c.Config.Instruments.PoolUtilization.Record(ctx, float64(busy)/float64(total), labels)
	c.Config.Instruments.QueueDepth.Record(ctx, int64(depth), labels)
}
