package metrics_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
)

func TestTheCatalogueDeclaresTheTenMandatorySeriesPlusTheTwoLocalOnes(t *testing.T) {
	want := []string{
		metrics.RetriesTotal,
		metrics.BudgetExhaustedTotal,
		metrics.BreakerState,
		metrics.DeadlineExceededTotal,
		metrics.CancellationsTotal,
		metrics.RequestDurationSeconds,
		metrics.RequestsTotal,
		metrics.ErrorsTotal,
		metrics.DegradedTotal,
		metrics.OmittedTotal,
		metrics.BulkheadRejectionsTotal,
		metrics.SpansDroppedTotal,
	}

	catalogue := metrics.Catalog()
	if len(catalogue) != len(want) {
		t.Fatalf("Catalog() has %d series, want %d", len(catalogue), len(want))
	}

	declared := make([]string, 0, len(catalogue))
	for _, m := range catalogue {
		declared = append(declared, m.Name)
	}
	for _, name := range want {
		if !slices.Contains(declared, name) {
			t.Errorf("series %q is missing from the catalogue (MET-02)", name)
		}
	}
}

func TestEverySeriesNameCarriesThePlatformPrefix(t *testing.T) {
	for _, m := range metrics.Catalog() {
		if !strings.HasPrefix(m.Name, "dmpf_") {
			t.Errorf("series %q does not carry the dmpf_ prefix", m.Name)
		}
	}
}

func TestEverySeriesDeclaresAFormulaAUnitAndAnOwner(t *testing.T) {
	for _, m := range metrics.Catalog() {
		if strings.TrimSpace(m.Formula) == "" {
			t.Errorf("series %q declares no formula: a number without one cannot be read (MET-03)", m.Name)
		}
		if m.Unit == "" {
			t.Errorf("series %q declares no unit", m.Name)
		}
		if m.Owner == "" {
			t.Errorf("series %q declares no owner", m.Name)
		}
		if len(m.Labels) == 0 {
			t.Errorf("series %q declares no labels", m.Name)
		}
	}
}

func TestOnlyTheBreakerStateDeclaresAThreshold(t *testing.T) {
	for _, m := range metrics.Catalog() {
		hasThreshold := m.Threshold != nil
		wantThreshold := m.Name == metrics.BreakerState

		if hasThreshold != wantThreshold {
			t.Errorf("series %q threshold present = %v, want %v — MET-05 derives one only for the breaker state",
				m.Name, hasThreshold, wantThreshold)
		}
		if wantThreshold && strings.TrimSpace(m.Threshold.Condition) == "" {
			t.Errorf("series %q declares an empty threshold condition", m.Name)
		}
	}
}

func TestTheDurationSeriesIsAHistogramInSeconds(t *testing.T) {
	for _, m := range metrics.Catalog() {
		if m.Name != metrics.RequestDurationSeconds {
			continue
		}
		if m.Kind != metrics.Histogram {
			t.Errorf("Kind = %q, want histogram", m.Kind)
		}
		if m.Unit != metrics.UnitSeconds {
			t.Errorf("Unit = %q, want %q", m.Unit, metrics.UnitSeconds)
		}
		return
	}
	t.Fatalf("series %q is missing from the catalogue", metrics.RequestDurationSeconds)
}

func TestTheBreakerStateIsAGauge(t *testing.T) {
	for _, m := range metrics.Catalog() {
		if m.Name == metrics.BreakerState && m.Kind != metrics.Gauge {
			t.Fatalf("Kind of %q = %q, want gauge", m.Name, m.Kind)
		}
	}
}

func TestCatalogReturnsACopy(t *testing.T) {
	metrics.Catalog()[0].Name = "tampered"

	if got := metrics.Catalog()[0].Name; got == "tampered" {
		t.Fatal("Catalog() shares its backing array: a caller rewrote the catalogue")
	}
}

func TestEveryLabelOfEverySeriesIsAPermittedKey(t *testing.T) {
	permitted := []string{
		metrics.KeyDependency, metrics.KeyOperation, metrics.KeyService,
		metrics.KeyErrorCategory, metrics.KeyOutcomeCategory,
	}

	for _, m := range metrics.Catalog() {
		for _, label := range m.Labels {
			if !slices.Contains(permitted, label) {
				t.Errorf("series %q declares label %q, which is not a permitted key (MET-04)", m.Name, label)
			}
		}
	}
}

func TestNewBuildsEverySeriesOnTheMeter(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() = %v, want nil", err)
		}
	})

	instruments, err := metrics.New(provider.Meter("dmpf-observability"))
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}

	labels := metrics.Labels{}.Dependency("payments").Operation("Authorize").Service("orders")
	instruments.Retries.Add(context.Background(), 1, measurement(labels))
	instruments.BreakerState.Record(context.Background(), 2, measurement(labels))
	instruments.RequestDuration.Record(context.Background(), 0.25, measurement(labels))

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v, want nil", err)
	}

	recorded := map[string]bool{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			recorded[m.Name] = true
		}
	}
	for _, name := range []string{metrics.RetriesTotal, metrics.BreakerState, metrics.RequestDurationSeconds} {
		if !recorded[name] {
			t.Errorf("series %q did not reach the reader", name)
		}
	}
}

func TestNewRefusesANilMeter(t *testing.T) {
	if _, err := metrics.New(nil); err == nil {
		t.Fatal("New(nil) = nil error, want a refusal: a nil meter would leave nil instruments")
	}
}
