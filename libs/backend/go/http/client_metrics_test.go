package http_test

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var semconvDurationBoundaries = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func TestEveryAttemptIsMeasuredInTheClientRequestDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(previous) })

	up := newUpstream(t, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK)
	client := newClient(t, clientConfig(route(http.MethodGet)))
	resp, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodGet, up.URL, ""))
	if err != nil {
		t.Fatalf("Do() = %v", err)
	}
	_ = resp.Body.Close()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	var duration *metricdata.Metrics
	for _, scope := range rm.ScopeMetrics {
		for i := range scope.Metrics {
			if scope.Metrics[i].Name == "http.client.request.duration" {
				duration = &scope.Metrics[i]
			}
		}
	}
	if duration == nil {
		t.Fatal("no http.client.request.duration series (RF-D2)")
	}
	if duration.Unit != "s" {
		t.Errorf("unit = %q, want s", duration.Unit)
	}
	histogram, ok := duration.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("aggregation = %T, want a float64 histogram", duration.Data)
	}
	byStatus := map[int64]uint64{}
	for _, point := range histogram.DataPoints {
		status, _ := point.Attributes.Value("http.response.status_code")
		byStatus[status.AsInt64()] += point.Count
		if method, _ := point.Attributes.Value("http.request.method"); method.AsString() != http.MethodGet {
			t.Errorf("http.request.method = %q, want GET", method.AsString())
		}
		if !slices.Equal(point.Bounds, semconvDurationBoundaries) {
			t.Errorf("bounds = %v, want the semconv advisory %v (RF-D4)", point.Bounds, semconvDurationBoundaries)
		}
	}
	if byStatus[http.StatusServiceUnavailable] != 2 || byStatus[http.StatusOK] != 1 || len(byStatus) != 2 {
		t.Fatalf("measured attempts by status = %v, want one per attempt the upstream saw (%d)", byStatus, up.count())
	}
}
