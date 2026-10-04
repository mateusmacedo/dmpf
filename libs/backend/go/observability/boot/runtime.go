package boot

import (
	"context"
	"sync"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func StartRuntimeMetrics(provider metric.MeterProvider) error {
	return runtime.Start(runtime.WithMeterProvider(provider))
}

var runtimeProducer sync.Once

// WHY: WithFallbackMetricProducer writes a package variable of autoexport
// that NewMetricReader reads (autoexport@v0.72.0/metrics.go:92-95,105).
func registerRuntimeProducer() {
	runtimeProducer.Do(func() {
		autoexport.WithFallbackMetricProducer(func(context.Context) (sdkmetric.Producer, error) {
			return runtime.NewProducer(), nil
		})
	})
}
