package otelboot_test

import (
	"context"
	"slices"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
	"go.opentelemetry.io/otel/semconv/v1.43.0/otelconv"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

func collectedKeys(t *testing.T, reader *sdkmetric.ManualReader, name string) map[string]bool {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok || len(sum.DataPoints) != 1 {
				t.Fatalf("%s is %T, want one int64 sum point", name, m.Data)
			}
			keys := map[string]bool{}
			for _, kv := range sum.DataPoints[0].Attributes.ToSlice() {
				keys[string(kv.Key)] = true
			}
			return keys
		}
	}
	t.Fatalf("%s did not reach the reader", name)
	return nil
}

func meteredRuntime(t *testing.T) (*otelboot.Runtime, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	runtime, _ := startedRuntime(t, func(config *otelboot.Config) { config.MetricReader = reader })
	return runtime, reader
}

var forbiddenOnAMetric = []attribute.KeyValue{
	semconv.ClientAddress("10.0.0.7"),
	semconv.NetworkPeerAddress("10.0.0.8"),
	semconv.NetworkPeerPort(5432),
	semconv.UserAgentOriginal("curl/8"),
	semconv.URLFull("https://pay.example/v1/cards?pan=4111"),
	semconv.URLPath("/v1/cards/4111"),
	semconv.ExceptionMessage("password authentication failed for user bookings"),
	semconv.ExceptionStacktrace("goroutine 1 [running]"),
}

func TestACatalogueSeriesKeepsOnlyTheLabelsItDeclares(t *testing.T) {
	runtime, reader := meteredRuntime(t)

	recorded := append([]attribute.KeyValue{
		attribute.String(metrics.KeyDependency, "payments"),
		attribute.String(metrics.KeyErrorType, "timeout"),
		attribute.String(metrics.KeyTenant, "acme"),
		attribute.String("service", "orders"),
	}, forbiddenOnAMetric...)
	runtime.Instruments().Retries.Add(context.Background(), 1, metric.WithAttributes(recorded...))

	keys := collectedKeys(t, reader, metrics.RetriesTotal)
	if len(keys) != 2 || !keys[metrics.KeyDependency] || !keys[metrics.KeyErrorType] {
		t.Fatalf("labels = %v, want only %s and %s: the View imposes the declared allowlist (RF-D3)",
			keys, metrics.KeyDependency, metrics.KeyErrorType)
	}
}

func TestASeriesOutsideTheCatalogueLosesTheForbiddenKeys(t *testing.T) {
	runtime, reader := meteredRuntime(t)
	counter, err := runtime.Meter().Int64Counter("http.server.request.count")
	if err != nil {
		t.Fatalf("Int64Counter() = %v", err)
	}

	recorded := append([]attribute.KeyValue{semconv.HTTPRoute("/orders/{id}")}, forbiddenOnAMetric...)
	counter.Add(context.Background(), 1, metric.WithAttributes(recorded...))

	keys := collectedKeys(t, reader, "http.server.request.count")
	if len(keys) != 1 || !keys[string(semconv.HTTPRouteKey)] {
		t.Fatalf("labels = %v, want only http.route: the keys of RF-B3 never leave the process (RF-D3)", keys)
	}
}

func TestTheViewKeepsTheUnitOfTheInstrument(t *testing.T) {
	runtime, reader := meteredRuntime(t)
	runtime.Instruments().Retries.Add(context.Background(), 1)

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == metrics.RetriesTotal {
				if m.Unit != metrics.UnitRetry {
					t.Fatalf("unit of %s = %q, want %q: the UCUM unit is declared on the instrument (RF-D1)",
						m.Name, m.Unit, metrics.UnitRetry)
				}
				return
			}
		}
	}
	t.Fatalf("%s did not reach the reader", metrics.RetriesTotal)
}

func histogramPoints(t *testing.T, reader *sdkmetric.ManualReader, name string) []attribute.Set {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			histogram, ok := m.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("%s is %T, want a float64 histogram", name, m.Data)
			}
			sets := make([]attribute.Set, len(histogram.DataPoints))
			for i, point := range histogram.DataPoints {
				sets[i] = point.Attributes
			}
			return sets
		}
	}
	t.Fatalf("%s did not reach the reader", name)
	return nil
}

func TestAnEdgeSeriesDoesNotSplitByTheHostTheClientSent(t *testing.T) {
	for _, name := range []string{
		httpconv.ServerRequestDuration{}.Name(),
		httpconv.ServerRequestBodySize{}.Name(),
		httpconv.ServerResponseBodySize{}.Name(),
	} {
		t.Run(name, func(t *testing.T) {
			runtime, reader := meteredRuntime(t)
			histogram, err := runtime.Meter().Float64Histogram(name)
			if err != nil {
				t.Fatalf("Float64Histogram() = %v", err)
			}

			for i, host := range []string{"evil-1", "evil-2", "evil-3"} {
				histogram.Record(context.Background(), 1, metric.WithAttributes(
					semconv.HTTPRoute("/orders/{id}"), semconv.ServerAddress(host), semconv.ServerPort(1000+i)))
			}

			points := histogramPoints(t, reader, name)
			if len(points) != 1 || points[0].HasValue(semconv.ServerAddressKey) || points[0].HasValue(semconv.ServerPortKey) {
				t.Fatalf("points = %v, want one series without server.address nor server.port: "+
					"otelhttp takes both from the Host header the client sends (RF-D3, MET-07)", points)
			}
		})
	}
}

func histogramBounds(t *testing.T, reader *sdkmetric.ManualReader, scope, name string) []float64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	for _, scoped := range collected.ScopeMetrics {
		if scoped.Scope.Name != scope {
			continue
		}
		for _, m := range scoped.Metrics {
			if m.Name != name {
				continue
			}
			histogram, ok := m.Data.(metricdata.Histogram[float64])
			if !ok || len(histogram.DataPoints) != 1 {
				t.Fatalf("%s is %T, want one float64 histogram point", name, m.Data)
			}
			return histogram.DataPoints[0].Bounds
		}
	}
	t.Fatalf("%s of %s did not reach the reader", name, scope)
	return nil
}

func TestTheSDKDurationHistogramsUseTheSemconvDurationBoundaries(t *testing.T) {
	want := []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
	collection := func(meter metric.Meter) (metric.Float64Histogram, error) {
		histogram, err := otelconv.NewSDKMetricReaderCollectionDuration(meter)
		return histogram.Inst(), err
	}
	export := func(meter metric.Meter) (metric.Float64Histogram, error) {
		histogram, err := otelconv.NewSDKExporterOperationDuration(meter)
		return histogram.Inst(), err
	}
	for _, emitter := range []struct {
		scope, version, name string
		histogram            func(metric.Meter) (metric.Float64Histogram, error)
	}{
		{"go.opentelemetry.io/otel/sdk/metric/internal/observ", "1.47.0",
			otelconv.SDKMetricReaderCollectionDuration{}.Name(), collection},
		{"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/observ", "1.47.0",
			otelconv.SDKExporterOperationDuration{}.Name(), export},
		{"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp/internal/observ", "1.47.0",
			otelconv.SDKExporterOperationDuration{}.Name(), export},
		{"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc", "1.47.0",
			otelconv.SDKExporterOperationDuration{}.Name(), export},
		{"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/observ", "1.47.0",
			otelconv.SDKExporterOperationDuration{}.Name(), export},
		{"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc/internal/observ", "0.23.0",
			otelconv.SDKExporterOperationDuration{}.Name(), export},
		{"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp/internal/observ", "0.23.0",
			otelconv.SDKExporterOperationDuration{}.Name(), export},
		{"go.opentelemetry.io/otel/exporters/stdout/stdouttrace/internal/observ", "1.47.0",
			otelconv.SDKExporterOperationDuration{}.Name(), export},
		{"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric/internal/observ", "1.47.0",
			otelconv.SDKExporterOperationDuration{}.Name(), export},
		{"go.opentelemetry.io/otel/exporters/stdout/stdoutlog/internal/observ", "v0.23.0",
			otelconv.SDKExporterOperationDuration{}.Name(), export},
		{"go.opentelemetry.io/otel/exporters/prometheus/internal/observ", "0.69.0",
			otelconv.SDKExporterOperationDuration{}.Name(), export},
		{"go.opentelemetry.io/otel/exporters/prometheus/internal/observ", "0.69.0",
			otelconv.SDKMetricReaderCollectionDuration{}.Name(), collection},
	} {
		t.Run(emitter.scope+"/"+emitter.name, func(t *testing.T) {
			runtime, reader := meteredRuntime(t)
			meter := runtime.MeterProvider().Meter(emitter.scope,
				metric.WithInstrumentationVersion(emitter.version), metric.WithSchemaURL(semconv.SchemaURL))
			histogram, err := emitter.histogram(meter)
			if err != nil {
				t.Fatalf("%s: %v", emitter.name, err)
			}
			histogram.Record(context.Background(), 0.2)

			if bounds := histogramBounds(t, reader, emitter.scope, emitter.name); !slices.Equal(bounds, want) {
				t.Fatalf("Bounds of %s = %v, want the semconv duration boundaries %v (RF-D4)", emitter.name, bounds, want)
			}
		})
	}
}

func TestTheViewKeepsTheBoundariesAnotherHistogramDeclares(t *testing.T) {
	runtime, reader := meteredRuntime(t)
	const scope, name = "dmpf.views.test", "dmpf.views.test.batch.size"
	want := []float64{1, 10, 100}
	histogram, err := runtime.MeterProvider().Meter(scope).Float64Histogram(name,
		metric.WithUnit("{record}"), metric.WithExplicitBucketBoundaries(want...))
	if err != nil {
		t.Fatalf("Float64Histogram() = %v", err)
	}
	histogram.Record(context.Background(), 5)

	if bounds := histogramBounds(t, reader, scope, name); !slices.Equal(bounds, want) {
		t.Fatalf("Bounds = %v, want the advisory %v: only the otel.sdk.* durations take boundaries from the View", bounds, want)
	}
}
