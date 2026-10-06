package metrics_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
)

func TestTheCatalogueDeclaresTheSeriesOfRFD1(t *testing.T) {
	want := map[string]string{
		metrics.RetriesTotal:             "dmpf.dependency.retries",
		metrics.BudgetExhaustedTotal:     "dmpf.dependency.budget.exhausted",
		metrics.BreakerState:             "dmpf.dependency.breaker.state",
		metrics.DeadlineExceededTotal:    "dmpf.dependency.deadline_exceeded",
		metrics.CancellationsTotal:       "dmpf.dependency.cancellations",
		metrics.BulkheadRejectionsTotal:  "dmpf.dependency.bulkhead.rejections",
		metrics.DegradedTotal:            "dmpf.dependency.degraded",
		metrics.OmittedTotal:             "dmpf.dependency.omitted",
		metrics.RequestDurationSeconds:   "dmpf.operation.duration",
		metrics.AdmissionRejectionsTotal: "dmpf.admission.rejections",
		metrics.PoolUtilization:          "dmpf.consumer.pool.utilization",
		metrics.QueueDepth:               "dmpf.consumer.queue.depth",
	}
	for constant, name := range want {
		if constant != name {
			t.Errorf("series %q, want %q (RF-D1)", constant, name)
		}
	}

	declared := map[string]bool{}
	for _, m := range metrics.Catalog() {
		declared[m.Name] = true
	}
	if len(declared) != len(want) {
		t.Errorf("Catalog() declares %d series, want the %d of RF-D1: %v", len(declared), len(want), declared)
	}
	for _, name := range want {
		if !declared[name] {
			t.Errorf("series %q is missing from the catalogue (RF-D1)", name)
		}
	}
}

func TestEverySeriesNameIsInTheOTelForm(t *testing.T) {
	for _, m := range metrics.Catalog() {
		if !strings.HasPrefix(m.Name, "dmpf.") {
			t.Errorf("series %q does not carry the dmpf. prefix", m.Name)
		}
		for _, suffix := range []string{"_total", "_seconds", "_ratio", "_count"} {
			if strings.HasSuffix(m.Name, suffix) {
				t.Errorf("series %q ends in %q: the exporter adds unit and type (RF-D1, MET-02)", m.Name, suffix)
			}
		}
		if strings.ToLower(m.Name) != m.Name {
			t.Errorf("series %q is not lower case", m.Name)
		}
	}
}

func TestEverySeriesDeclaresItsUCUMUnit(t *testing.T) {
	want := map[string]string{
		metrics.RetriesTotal:             "{retry}",
		metrics.BudgetExhaustedTotal:     "{execution}",
		metrics.BreakerState:             "{state}",
		metrics.DeadlineExceededTotal:    "{call}",
		metrics.CancellationsTotal:       "{call}",
		metrics.BulkheadRejectionsTotal:  "{call}",
		metrics.DegradedTotal:            "{response}",
		metrics.OmittedTotal:             "{response}",
		metrics.RequestDurationSeconds:   "s",
		metrics.AdmissionRejectionsTotal: "{request}",
		metrics.PoolUtilization:          "1",
		metrics.QueueDepth:               "{message}",
	}
	for _, m := range metrics.Catalog() {
		if m.Unit != want[m.Name] {
			t.Errorf("unit of %q = %q, want %q (RF-D1: UCUM, 1 only for a ratio)", m.Name, m.Unit, want[m.Name])
		}
	}
}

func TestEverySeriesDeclaresAFormulaAndAnOwner(t *testing.T) {
	for _, m := range metrics.Catalog() {
		if strings.TrimSpace(m.Formula) == "" {
			t.Errorf("series %q declares no formula: a number without one cannot be read (MET-03)", m.Name)
		}
		if m.Owner == "" {
			t.Errorf("series %q declares no owner", m.Name)
		}
	}
}

func TestEverySeriesDeclaresTheLabelsOfRFD3(t *testing.T) {
	want := map[string][]string{
		metrics.RetriesTotal:             {"dmpf.dependency", "error.type"},
		metrics.BudgetExhaustedTotal:     {"dmpf.dependency"},
		metrics.BreakerState:             {"dmpf.dependency"},
		metrics.DeadlineExceededTotal:    {"dmpf.dependency", "dmpf.operation"},
		metrics.CancellationsTotal:       {"dmpf.dependency", "dmpf.operation"},
		metrics.BulkheadRejectionsTotal:  {"dmpf.dependency"},
		metrics.DegradedTotal:            {"dmpf.dependency"},
		metrics.OmittedTotal:             {"dmpf.dependency"},
		metrics.RequestDurationSeconds:   {"dmpf.operation", "dmpf.outcome_category", "error.type"},
		metrics.AdmissionRejectionsTotal: {"http.route", "rpc.method", "dmpf.tenant_id"},
		metrics.PoolUtilization:          nil,
		metrics.QueueDepth:               nil,
	}
	for _, m := range metrics.Catalog() {
		if !slices.Equal(m.Labels, want[m.Name]) {
			t.Errorf("labels of %q = %v, want %v (RF-D3: service is a resource attribute)", m.Name, m.Labels, want[m.Name])
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

func TestTheStateAndSaturationSeriesAreGauges(t *testing.T) {
	gauges := []string{metrics.BreakerState, metrics.PoolUtilization, metrics.QueueDepth}
	for _, m := range metrics.Catalog() {
		if slices.Contains(gauges, m.Name) && m.Kind != metrics.Gauge {
			t.Errorf("Kind of %q = %q, want gauge (MET-11)", m.Name, m.Kind)
		}
	}
}

func TestTheAdmissionSeriesCarriesRouteAndTenantOnly(t *testing.T) {
	for _, m := range metrics.Catalog() {
		if m.Name != metrics.AdmissionRejectionsTotal {
			continue
		}
		if !slices.Equal(m.Labels, []string{metrics.KeyRoute, metrics.KeyRPCMethod, metrics.KeyTenant}) {
			t.Fatalf("Labels = %v, want [http.route rpc.method dmpf.tenant_id] (MET-12, RF-D3)", m.Labels)
		}
		return
	}
	t.Fatalf("series %q is missing from the catalogue", metrics.AdmissionRejectionsTotal)
}

func TestCatalogReturnsACopy(t *testing.T) {
	metrics.Catalog()[0].Name = "tampered"

	if got := metrics.Catalog()[0].Name; got == "tampered" {
		t.Fatal("Catalog() shares its backing array: a caller rewrote the catalogue")
	}
}

func TestEveryLabelOfEverySeriesIsAPermittedKey(t *testing.T) {
	permitted := []string{
		metrics.KeyDependency, metrics.KeyOperation,
		metrics.KeyErrorType, metrics.KeyOutcomeCategory,
		metrics.KeyRoute, metrics.KeyRPCMethod, metrics.KeyTenant,
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

	instruments, err := metrics.New(provider.Meter("observability"))
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}

	labels := metrics.Labels{}.Dependency("payments").Operation("Authorize")
	instruments.Retries.Add(context.Background(), 1, measurement(labels))
	instruments.BreakerState.Record(context.Background(), 2, measurement(labels))
	instruments.RequestDuration.Record(context.Background(), 0.25, measurement(labels))
	instruments.PoolUtilization.Record(context.Background(), 0.5, measurement(labels))
	instruments.QueueDepth.Record(context.Background(), 3, measurement(labels))
	instruments.AdmissionRejections.Add(context.Background(), 1, measurement(labels))

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
	for _, name := range []string{
		metrics.RetriesTotal, metrics.BreakerState, metrics.RequestDurationSeconds,
		metrics.PoolUtilization, metrics.QueueDepth, metrics.AdmissionRejectionsTotal,
	} {
		if !recorded[name] {
			t.Errorf("series %q did not reach the reader", name)
		}
	}
}

type namingMeter struct {
	noop.Meter
	names []string
}

func (m *namingMeter) Int64Counter(name string, options ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	m.names = append(m.names, name)
	return m.Meter.Int64Counter(name, options...)
}

func (m *namingMeter) Int64Gauge(name string, options ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	m.names = append(m.names, name)
	return m.Meter.Int64Gauge(name, options...)
}

func (m *namingMeter) Float64Gauge(name string, options ...metric.Float64GaugeOption) (metric.Float64Gauge, error) {
	m.names = append(m.names, name)
	return m.Meter.Float64Gauge(name, options...)
}

func (m *namingMeter) Float64Histogram(name string, options ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	m.names = append(m.names, name)
	return m.Meter.Float64Histogram(name, options...)
}

func TestNewBuildsOnlyTheSeriesOfTheCatalogue(t *testing.T) {
	meter := &namingMeter{}
	if _, err := metrics.New(meter); err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}

	declared := map[string]bool{}
	for _, m := range metrics.Catalog() {
		declared[m.Name] = true
	}
	for _, name := range meter.names {
		if !declared[name] {
			t.Errorf("New() builds %q, which is not in the catalogue (RF-D1, RF-D2)", name)
		}
	}
	if len(meter.names) != len(declared) {
		t.Errorf("New() builds %d series, want the %d of the catalogue: %v", len(meter.names), len(declared), meter.names)
	}
}

func TestNewRefusesANilMeter(t *testing.T) {
	if _, err := metrics.New(nil); err == nil {
		t.Fatal("New(nil) = nil error, want a refusal: a nil meter would leave nil instruments")
	}
}
