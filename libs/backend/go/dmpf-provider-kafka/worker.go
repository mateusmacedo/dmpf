// comment-discipline-ok-file: arquivo de contrato interno; cada godoc cita a regra de FND-06 (KFK-09, KFK-10, TRP-29, TRP-47, TRP-48) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfkafka

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"sync/atomic"

	"github.com/twmb/franz-go/pkg/kgo"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/attempt"
)

// partitionWorker processes the records of one partition in order (KFK-09):
// one goroutine, one queue, one cursor, cancelled on revocation (TRP-48). A
// record left without disposition stalls it: the next would apply out of order.
type partitionWorker struct {
	consumer *Consumer
	client   client
	key      key
	ctx      context.Context
	cancel   context.CancelFunc
	signal   chan struct{}
	done     chan struct{}
	busy     atomic.Bool

	mu      sync.Mutex
	queue   []*kgo.Record
	paused  bool
	stalled bool
	revoked bool
	cursor  cursor
}

func newPartitionWorker(ctx context.Context, c *Consumer, cl client, k key) *partitionWorker {
	ctx, cancel := context.WithCancel(ctx)
	w := &partitionWorker{
		consumer: c,
		client:   cl,
		key:      k,
		ctx:      ctx,
		cancel:   cancel,
		signal:   make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	go w.run()
	return w
}

// enqueue hands a record to the worker without blocking the poll loop, so the
// rebalance is never held by a slow partition; above the declared capacity the
// partition's fetch is paused until the queue drains.
func (w *partitionWorker) enqueue(record *kgo.Record) {
	w.mu.Lock()
	w.queue = append(w.queue, record)
	pause := !w.paused && len(w.queue) >= w.consumer.QueuePerPartition
	if pause {
		w.paused = true
	}
	w.mu.Unlock()
	if pause {
		w.client.PauseFetchPartitions(w.partitions())
	}
	select {
	case w.signal <- struct{}{}:
	default:
	}
}

func (w *partitionWorker) partitions() map[string][]int32 {
	return map[string][]int32{w.key.topic: {w.key.partition}}
}

// markRevoked closes the commit gate before the cancellation: a worker that
// acknowledged between the check of its context and the commit does not commit
// a partition that is no longer its own; the revocation commits what is due.
func (w *partitionWorker) markRevoked() {
	w.mu.Lock()
	w.revoked = true
	w.mu.Unlock()
}

func (w *partitionWorker) stop() {
	w.cancel()
	<-w.done
}

func (w *partitionWorker) run() {
	defer close(w.done)
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.signal:
		}
		for {
			if w.ctx.Err() != nil {
				return
			}
			record, resume, ok := w.next()
			if resume {
				w.client.ResumeFetchPartitions(w.partitions())
			}
			if !ok {
				break
			}
			w.busy.Store(true)
			w.process(record)
			w.busy.Store(false)
		}
	}
}

// next pops the head of the queue unless the partition is stalled; it also
// says whether the fetch can resume, once the queue drained below capacity.
func (w *partitionWorker) next() (record *kgo.Record, resume bool, ok bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stalled || len(w.queue) == 0 {
		return nil, false, false
	}
	record = w.queue[0]
	w.queue[0] = nil
	w.queue = w.queue[1:]
	if w.paused && !w.stalled && len(w.queue) < w.consumer.QueuePerPartition/2 {
		w.paused = false
		resume = true
	}
	return record, resume, true
}

// process runs the inline retry of KFK-10 on one record: Ack marks and commits,
// Release waits out the backoff with the partition paused; the channel's limit
// or a return without gesture leaves it pending and stalls the partition (TRP-47).
func (w *partitionWorker) process(record *kgo.Record) {
	w.mu.Lock()
	w.cursor.Track(record)
	w.mu.Unlock()

	limit := w.consumer.Channel.Retry.MaxAttempts
	c := w.consumer.Config.Clock
	random := w.consumer.Config.Rand
	if random == nil {
		random = rand.Float64
	}

	for attemptNo := 1; ; attemptNo++ {
		if w.ctx.Err() != nil {
			return
		}
		ack := &acknowledger{}
		attemptCtx, cancel := c.WithTimeout(w.ctx, w.consumer.ProcessingDeadline)
		err := w.handle(attemptCtx, record, attemptNo, ack)
		cancel()

		if w.ctx.Err() != nil {
			return
		}

		switch ack.decision() {
		case acked:
			if err != nil {
				w.consumer.Config.logger().WarnContext(w.ctx, "dmpfkafka: sink acknowledged with an error",
					slog.String("topic", w.key.topic), slog.Int("partition", int(w.key.partition)), slog.Int64("offset", record.Offset),
					slog.Int("attempt", attemptNo), slog.String("error_category", categoryOf(err)))
			}
			w.mu.Lock()
			w.cursor.Mark(record.Offset)
			w.mu.Unlock()
			w.commitContiguous(w.ctx, w.client, false)
			return
		case undecided:
			w.stall(record, attemptNo, "sink returned without a gesture; record left pending and partition stalled", err)
			return
		}

		if attemptNo >= limit {
			w.stall(record, attemptNo, "attempts exhausted; record left pending for the adapter's containment and partition stalled", err)
			return
		}

		wait := w.consumer.Backoff.Next(attemptNo, random)
		w.pauseForBackoff()
		sleepErr := clock.NewSleeper(c)(w.ctx, wait)
		w.resumeAfterBackoff()
		if sleepErr != nil {
			return
		}
	}
}

// handle runs the sink on one attempt. A panic inside it — the sink decodes
// bytes from outside the process — is a failed attempt without a gesture, not
// the end of every worker of the consumer.
func (w *partitionWorker) handle(ctx context.Context, record *kgo.Record, attemptNo int, ack *acknowledger) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: %v", ErrSinkPanicked, recovered)
		}
	}()
	return w.consumer.Sink.Handle(attempt.WithContext(ctx, attemptNo), record.Value, attemptNo, ack)
}

// resumeFetch lifts the fetch pause a partition may carry when it leaves this
// worker: kgo keeps paused partitions across rebalances, so a stall or a backoff
// pause would otherwise follow the partition into its next assignment.
func (w *partitionWorker) resumeFetch() {
	w.mu.Lock()
	paused := w.paused
	w.paused = false
	w.mu.Unlock()
	if paused {
		w.client.ResumeFetchPartitions(w.partitions())
	}
}

// stall leaves the partition paused and its queue untouched: nothing after a
// record without disposition is processed, so no effect lands out of order.
// Only a revocation releases the partition, to whoever receives it next.
func (w *partitionWorker) stall(record *kgo.Record, attemptNo int, why string, cause error) {
	w.mu.Lock()
	w.stalled = true
	pause := !w.paused
	w.paused = true
	queued := len(w.queue)
	w.mu.Unlock()
	if pause {
		w.client.PauseFetchPartitions(w.partitions())
	}
	w.consumer.Config.logger().ErrorContext(w.ctx, "dmpfkafka: "+why,
		slog.String("topic", w.key.topic), slog.Int("partition", int(w.key.partition)), slog.Int64("offset", record.Offset),
		slog.Int("attempts", attemptNo), slog.Int("queued_behind", queued), slog.String("error_category", categoryOf(cause)))
}

func (w *partitionWorker) pauseForBackoff() {
	w.mu.Lock()
	pause := !w.paused
	w.paused = true
	w.mu.Unlock()
	if pause {
		w.client.PauseFetchPartitions(w.partitions())
	}
}

func (w *partitionWorker) resumeAfterBackoff() {
	w.mu.Lock()
	resume := w.paused && !w.stalled && len(w.queue) < w.consumer.QueuePerPartition
	if resume {
		w.paused = false
	}
	w.mu.Unlock()
	if resume {
		w.client.ResumeFetchPartitions(w.partitions())
	}
}

// commitContiguous commits one past the last contiguously acknowledged record
// (TRP-29). The worker itself does not commit once the partition was revoked;
// the revocation, with final set, commits what was acknowledged before it.
func (w *partitionWorker) commitContiguous(ctx context.Context, cl client, final bool) {
	w.mu.Lock()
	if w.revoked && !final {
		w.mu.Unlock()
		return
	}
	record, ok := w.cursor.NextContiguous()
	w.mu.Unlock()
	if !ok {
		return
	}
	if err := cl.CommitRecords(ctx, record); err != nil {
		w.consumer.Config.logger().ErrorContext(ctx, "dmpfkafka: offset commit failed",
			slog.String("topic", w.key.topic), slog.Int("partition", int(w.key.partition)),
			slog.Int64("offset", record.Offset), slog.String("error_category", categoryOf(err)))
	}
}

// Pending reports the records of the partition tracked and not yet committed.
func (w *partitionWorker) Pending() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cursor.Pending()
}

// Queued reports the records waiting in the worker's queue.
func (w *partitionWorker) Queued() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.queue)
}

// Stalled reports whether a record without disposition stopped the partition.
func (w *partitionWorker) Stalled() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stalled
}
