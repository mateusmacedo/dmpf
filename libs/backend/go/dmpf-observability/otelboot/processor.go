package otelboot

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ErrExporterRequired is a processor built without an exporter to own.
var ErrExporterRequired = errors.New("otelboot: the processor requires a span exporter")

// Defaults of the queue. The queue holds what the exporter has not taken yet;
// the batch is how much of it goes out in one call.
const (
	DefaultQueueSize = 2048
	DefaultBatchSize = 512
)

// ProcessorOptions tunes the queue. The zero value is the platform default.
type ProcessorOptions struct {
	QueueSize int
	BatchSize int

	// Dropped counts the spans the queue had to let go. It is optional so the
	// processor can be built before the meter exists, but a runtime without it
	// loses the only evidence that telemetry was shed.
	Dropped metric.Int64Counter
}

// ClassAwareProcessor is the single SpanProcessor of the platform and the sole
// owner of the SpanExporter. It exports every span that is sampled or ends in
// error, which is the in-process rule TRC-14 asks for and no factory processor
// gives: BatchSpanProcessor and SimpleSpanProcessor both discard a span whose
// FlagsSampled is clear (sdk/trace/batch_span_processor.go:403), so an
// unsampled failure would be lost.
//
// It never shares the exporter, because ExportSpans is synchronous and gives no
// concurrency guarantee (sdk/trace/span_exporter.go:16-19).
type ClassAwareProcessor struct {
	exporter  sdktrace.SpanExporter
	dropped   metric.Int64Counter
	batchSize int
	capacity  int

	mu      sync.Mutex
	queue   []sdktrace.ReadOnlySpan
	pending error

	wake  chan struct{}
	flush chan chan error
	stop  chan struct{}
	done  chan struct{}

	stopOnce sync.Once
	stopped  chan struct{}
}

// NewClassAwareProcessor starts the worker that owns the exporter. The caller
// closes it with Shutdown.
func NewClassAwareProcessor(exporter sdktrace.SpanExporter, options ProcessorOptions) (*ClassAwareProcessor, error) {
	if exporter == nil {
		return nil, ErrExporterRequired
	}

	capacity := options.QueueSize
	if capacity <= 0 {
		capacity = DefaultQueueSize
	}
	batchSize := options.BatchSize
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}

	processor := &ClassAwareProcessor{
		exporter:  exporter,
		dropped:   options.Dropped,
		batchSize: batchSize,
		capacity:  capacity,
		queue:     make([]sdktrace.ReadOnlySpan, 0, capacity),
		wake:      make(chan struct{}, 1),
		flush:     make(chan chan error),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
	go processor.run()
	return processor, nil
}

// OnStart records nothing: the decision of what to export is taken when the
// span ends and its status is known.
func (p *ClassAwareProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}

// OnEnd queues a span that is sampled or failed. It returns without waiting for
// the exporter, as the SpanProcessor contract requires
// (sdk/trace/span_processor.go:25-27).
func (p *ClassAwareProcessor) OnEnd(span sdktrace.ReadOnlySpan) {
	if span == nil || !worthExporting(span) {
		return
	}
	select {
	case <-p.stopped:
		return
	default:
	}

	if shed := p.enqueue(span); shed > 0 {
		p.countDropped(shed)
	}
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// worthExporting is the rule of TRC-14: a sampled span is telemetry the
// platform asked for, and a failed span is telemetry it cannot afford to lose,
// sampled or not.
func worthExporting(span sdktrace.ReadOnlySpan) bool {
	return span.SpanContext().IsSampled() || span.Status().Code == codes.Error
}

// enqueue appends the span and reports how many had to be shed. A full queue
// gives up the oldest span that carries no error; when every queued span is a
// failure, the new span is the one refused. Either way the loss is counted, so
// a gap in the traces is never silent.
func (p *ClassAwareProcessor) enqueue(span sdktrace.ReadOnlySpan) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.queue) < p.capacity {
		p.queue = append(p.queue, span)
		return 0
	}

	for i, queued := range p.queue {
		if queued.Status().Code != codes.Error {
			p.queue = append(p.queue[:i], p.queue[i+1:]...)
			p.queue = append(p.queue, span)
			return 1
		}
	}
	return 1
}

func (p *ClassAwareProcessor) countDropped(shed int) {
	if p.dropped == nil {
		return
	}
	p.dropped.Add(context.Background(), int64(shed))
}

func (p *ClassAwareProcessor) run() {
	defer close(p.done)

	for {
		select {
		case <-p.stop:
			p.retain(p.drain(context.Background()))
			return
		case reply := <-p.flush:
			drained := p.drain(context.Background())
			reply <- errors.Join(p.takePending(), drained)
			p.rewake()
		case <-p.wake:
			p.retain(p.drain(context.Background()))
			p.rewake()
		}
	}
}

// drain exports what was queued when it started, one batch at a time, and joins
// whatever the exporter reported so a caller of ForceFlush learns of a failure.
//
// The bound matters: draining until the queue is empty would never return while
// another goroutine keeps ending spans, and ForceFlush and Shutdown would hang
// under exactly the load that makes them worth calling. What arrives afterwards
// is the next cycle's work.
func (p *ClassAwareProcessor) drain(ctx context.Context) error {
	var failures []error
	for remaining := p.queued(); remaining > 0; {
		batch := p.take(min(remaining, p.batchSize))
		if len(batch) == 0 {
			break
		}
		remaining -= len(batch)

		if err := p.exporter.ExportSpans(ctx, batch); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// rewake asks for another cycle when spans arrived while the last one ran, so a
// bounded drain never leaves work sitting in the queue.
func (p *ClassAwareProcessor) rewake() {
	if p.queued() == 0 {
		return
	}
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *ClassAwareProcessor) queued() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.queue)
}

// retain holds a failure the worker met on its own, so the next ForceFlush or
// Shutdown reports it. An export that failed between flushes would otherwise be
// a loss nobody hears about.
func (p *ClassAwareProcessor) retain(err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pending = errors.Join(p.pending, err)
}

func (p *ClassAwareProcessor) takePending() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	pending := p.pending
	p.pending = nil
	return pending
}

func (p *ClassAwareProcessor) take(want int) []sdktrace.ReadOnlySpan {
	p.mu.Lock()
	defer p.mu.Unlock()

	size := min(len(p.queue), want)
	if size <= 0 {
		return nil
	}
	batch := make([]sdktrace.ReadOnlySpan, size)
	copy(batch, p.queue[:size])

	// Shift what is left to the front and clear the tail: the backing array
	// outlives the slice, so a span left behind it would be held for as long as
	// the queue exists.
	kept := copy(p.queue, p.queue[size:])
	clear(p.queue[kept:])
	p.queue = p.queue[:kept]

	return batch
}

// ForceFlush exports what is queued and waits for the worker to finish. It
// honours the caller's context rather than outliving it.
func (p *ClassAwareProcessor) ForceFlush(ctx context.Context) error {
	reply := make(chan error, 1)
	select {
	case p.flush <- reply:
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown stops the worker, drains what is left and closes the exporter, in
// that order. It is idempotent, and a span that arrives afterwards is ignored
// instead of queued into a processor nobody will read.
func (p *ClassAwareProcessor) Shutdown(ctx context.Context) error {
	p.stopOnce.Do(func() {
		close(p.stopped)
		close(p.stop)
	})

	select {
	case <-p.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return errors.Join(p.takePending(), p.exporter.Shutdown(ctx))
}
