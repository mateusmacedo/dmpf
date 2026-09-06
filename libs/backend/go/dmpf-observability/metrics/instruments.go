package metrics

import (
	"fmt"

	"go.opentelemetry.io/otel/metric"
)

// Instruments is the platform catalogue realized over a meter. It is built once
// at boot: an instrument created per call would re-register the same series.
type Instruments struct {
	Retries             metric.Int64Counter
	BudgetExhausted     metric.Int64Counter
	BreakerState        metric.Int64Gauge
	DeadlineExceeded    metric.Int64Counter
	Cancellations       metric.Int64Counter
	RequestDuration     metric.Float64Histogram
	Requests            metric.Int64Counter
	Errors              metric.Int64Counter
	Degraded            metric.Int64Counter
	Omitted             metric.Int64Counter
	BulkheadRejections  metric.Int64Counter
	SpansDropped        metric.Int64Counter
	PoolUtilization     metric.Float64Gauge
	QueueDepth          metric.Int64Gauge
	AdmissionRejections metric.Int64Counter
}

// New builds every series of Catalog on the meter. It fails as a whole: a
// partially built set would leave a decorator recording into a nil instrument.
func New(meter metric.Meter) (*Instruments, error) {
	if meter == nil {
		return nil, fmt.Errorf("metrics: a meter is required")
	}

	byName := make(map[string]Metric, len(Catalog()))
	for _, m := range Catalog() {
		byName[m.Name] = m
	}

	var failure error
	counter := func(name string) metric.Int64Counter {
		declared := byName[name]
		instrument, err := meter.Int64Counter(name,
			metric.WithUnit(declared.Unit),
			metric.WithDescription(declared.Formula))
		if err != nil && failure == nil {
			failure = fmt.Errorf("metrics: %s: %w", name, err)
		}
		return instrument
	}

	instruments := &Instruments{
		Retries:             counter(RetriesTotal),
		BudgetExhausted:     counter(BudgetExhaustedTotal),
		DeadlineExceeded:    counter(DeadlineExceededTotal),
		Cancellations:       counter(CancellationsTotal),
		Requests:            counter(RequestsTotal),
		Errors:              counter(ErrorsTotal),
		Degraded:            counter(DegradedTotal),
		Omitted:             counter(OmittedTotal),
		BulkheadRejections:  counter(BulkheadRejectionsTotal),
		SpansDropped:        counter(SpansDroppedTotal),
		AdmissionRejections: counter(AdmissionRejectionsTotal),
	}

	gauge := byName[BreakerState]
	state, err := meter.Int64Gauge(BreakerState,
		metric.WithUnit(gauge.Unit),
		metric.WithDescription(gauge.Formula))
	if err != nil && failure == nil {
		failure = fmt.Errorf("metrics: %s: %w", BreakerState, err)
	}
	instruments.BreakerState = state

	duration := byName[RequestDurationSeconds]
	histogram, err := meter.Float64Histogram(RequestDurationSeconds,
		metric.WithUnit(duration.Unit),
		metric.WithDescription(duration.Formula))
	if err != nil && failure == nil {
		failure = fmt.Errorf("metrics: %s: %w", RequestDurationSeconds, err)
	}
	instruments.RequestDuration = histogram

	utilization := byName[PoolUtilization]
	pool, err := meter.Float64Gauge(PoolUtilization,
		metric.WithUnit(utilization.Unit),
		metric.WithDescription(utilization.Formula))
	if err != nil && failure == nil {
		failure = fmt.Errorf("metrics: %s: %w", PoolUtilization, err)
	}
	instruments.PoolUtilization = pool

	depth := byName[QueueDepth]
	queue, err := meter.Int64Gauge(QueueDepth,
		metric.WithUnit(depth.Unit),
		metric.WithDescription(depth.Formula))
	if err != nil && failure == nil {
		failure = fmt.Errorf("metrics: %s: %w", QueueDepth, err)
	}
	instruments.QueueDepth = queue

	if failure != nil {
		return nil, failure
	}
	return instruments, nil
}
