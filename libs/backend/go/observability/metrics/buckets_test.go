package metrics_test

import (
	"context"
	"slices"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
)

// The histogram is declared in seconds (UnitSeconds) and the instrumentation
// records elapsed.Seconds(); the SDK's default boundaries (5, 10, 25 … 10000)
// were drawn for milliseconds, so every request of a healthy service would land
// in the first bucket and histogram_quantile would report seconds for a
// millisecond call. The boundaries have to be declared on the seconds scale.
func TestRequestDurationBucketsAreOnTheSecondsScale(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	instruments, err := metrics.New(provider.Meter("observability"))
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}
	labels := metrics.Labels{}.Operation("orders.FindOrder")
	instruments.RequestDuration.Record(context.Background(), 0.003, measurement(labels))

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v, want nil", err)
	}

	var point metricdata.HistogramDataPoint[float64]
	found := false
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != metrics.RequestDurationSeconds {
				continue
			}
			hist, ok := m.Data.(metricdata.Histogram[float64])
			if !ok || len(hist.DataPoints) != 1 {
				t.Fatalf("%s is %T with %d points, want one float64 histogram point", m.Name, m.Data, len(hist.DataPoints))
			}
			point, found = hist.DataPoints[0], true
		}
	}
	if !found {
		t.Fatalf("%s did not reach the reader", metrics.RequestDurationSeconds)
	}

	if len(point.Bounds) == 0 || point.Bounds[0] > 0.01 {
		t.Fatalf("first boundary = %v, want a sub-10ms bucket: the histogram is in seconds", point.Bounds)
	}
	if last := point.Bounds[len(point.Bounds)-1]; last > 60 {
		t.Fatalf("last boundary = %v s, want at most a minute: anything above is not a request", last)
	}

	// 3 ms must not share a bucket with 4 s: find the bucket that took the
	// sample and require its upper bound to be below one second.
	for i, count := range point.BucketCounts {
		if count == 0 {
			continue
		}
		if i >= len(point.Bounds) {
			t.Fatalf("the 3 ms sample fell in the overflow bucket (> %v s)", point.Bounds[len(point.Bounds)-1])
		}
		if point.Bounds[i] >= 1 {
			t.Fatalf("the 3 ms sample fell in the bucket ending at %v s, want a sub-second bucket", point.Bounds[i])
		}
	}
}

func TestTheDurationBucketsAreTheSemconvAdvisoryBoundaries(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	instruments, err := metrics.New(provider.Meter("observability"))
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}
	instruments.RequestDuration.Record(context.Background(), 0.2)

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v, want nil", err)
	}
	want := []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != metrics.RequestDurationSeconds {
				continue
			}
			bounds := m.Data.(metricdata.Histogram[float64]).DataPoints[0].Bounds
			if !slices.Equal(bounds, want) {
				t.Fatalf("Bounds = %v, want the semconv advisory %v (RF-D4)", bounds, want)
			}
			return
		}
	}
	t.Fatalf("%s did not reach the reader", metrics.RequestDurationSeconds)
}
