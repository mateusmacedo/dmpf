package boot_test

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
)

func TestTheGoRuntimeMetricsReachTheMeterProviderOfTheProcess(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	if err := boot.StartRuntimeMetrics(provider); err != nil {
		t.Fatalf("StartRuntimeMetrics() = %v", err)
	}

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	found := map[string]bool{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			found[m.Name] = true
		}
	}
	for _, name := range []string{"go.goroutine.count", "go.memory.used"} {
		if !found[name] {
			t.Errorf("%s did not reach the reader (RF-D5); got %v", name, found)
		}
	}
}
