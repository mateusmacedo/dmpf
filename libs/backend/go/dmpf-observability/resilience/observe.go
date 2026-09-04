package resilience

import (
	"context"

	"go.opentelemetry.io/otel/metric"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
)

// metric64Counter is the slice of metric.Int64Counter the decorators use. It is
// named here so a decorator accepts a nil instrument and a test exercises the
// decision without a meter.
type metric64Counter interface {
	Add(ctx context.Context, incr int64, options ...metric.AddOption)
}

// metric64Gauge is the same for a gauge, which the breaker records its state on.
type metric64Gauge interface {
	Record(ctx context.Context, value int64, options ...metric.RecordOption)
}

func metricAttributes(labels metrics.Labels) metric.MeasurementOption {
	return metric.WithAttributes(labels.Attributes()...)
}
