package obstest

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

func MetricLabels(t testing.TB, reader *sdkmetric.ManualReader) map[string][]attribute.Set {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	labels := map[string][]attribute.Set{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Gauge[float64]:
				for _, point := range data.DataPoints {
					labels[m.Name] = append(labels[m.Name], point.Attributes)
				}
			case metricdata.Gauge[int64]:
				for _, point := range data.DataPoints {
					labels[m.Name] = append(labels[m.Name], point.Attributes)
				}
			}
		}
	}
	return labels
}

func AwaitSpan(t testing.TB, runtime *otelboot.Runtime, spans *tracetest.InMemoryExporter, prefix string, timeout time.Duration) tracetest.SpanStub {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if err := runtime.ForceFlush(context.Background()); err != nil {
			t.Fatalf("ForceFlush() = %v, want nil", err)
		}
		names := []string{}
		for _, span := range spans.GetSpans() {
			if strings.HasPrefix(span.Name, prefix) {
				return span
			}
			names = append(names, span.Name)
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s span within %v, exported %v", prefix, timeout, names)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func MeteredRuntime(t testing.TB, service, role string, reader sdkmetric.Reader) *otelboot.Runtime {
	t.Helper()
	runtime, err := otelboot.Start(context.Background(), otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: service, ServiceVersion: "dev", ServiceInstanceID: service + "-" + role + "-1", Role: role},
		TraceExporter: tracetest.NewInMemoryExporter(),
		MetricReader:  reader,
	})
	if err != nil {
		t.Fatalf("otelboot.Start() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	return runtime
}

func AwaitPooledConnection(t testing.TB, reader *sdkmetric.ManualReader, dsn string, stopped <-chan error, timeout time.Duration) {
	t.Helper()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig() = %v, want nil", err)
	}
	pool := config.ConnConfig.Host + ":" + strconv.Itoa(int(config.ConnConfig.Port)) + "/" + config.ConnConfig.Database
	deadline := time.Now().Add(timeout)
	for PooledConnections(t, reader, pool) < 1 {
		select {
		case err := <-stopped:
			t.Fatalf("RunWith() = %v before the runtime reported a connection of the pool %s", err, pool)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("db.client.connection.count reported no connection of the pool %s within %v: the role builds its pool without the meter provider of the runtime (RF-D6)", pool, timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func PooledConnections(t testing.TB, reader *sdkmetric.ManualReader, pool string) int64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v, want nil", err)
	}
	var connections int64
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			count, ok := m.Data.(metricdata.Sum[int64])
			if m.Name != "db.client.connection.count" || !ok {
				continue
			}
			for _, point := range count.DataPoints {
				if name, found := point.Attributes.Value(semconv.DBClientConnectionPoolNameKey); found && name.AsString() == pool {
					connections += point.Value
				}
			}
		}
	}
	return connections
}

func SpanErrorType(t testing.TB, spans *tracetest.InMemoryExporter, name string) string {
	t.Helper()
	for _, span := range spans.GetSpans() {
		if span.Name != name {
			continue
		}
		for _, kv := range span.Attributes {
			if string(kv.Key) == metrics.KeyErrorType {
				return kv.Value.AsString()
			}
		}
		t.Fatalf("span %s carries no %s: %v", span.Name, metrics.KeyErrorType, span.Attributes)
	}
	t.Fatalf("spans = %v, want %s", spans.GetSpans().Snapshots(), name)
	return ""
}

func REDErrorType(t testing.TB, reader *sdkmetric.ManualReader, operation string) string {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v, want nil", err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			histogram, ok := m.Data.(metricdata.Histogram[float64])
			if m.Name != metrics.RequestDurationSeconds || !ok {
				continue
			}
			for _, point := range histogram.DataPoints {
				if value, _ := point.Attributes.Value(attribute.Key(metrics.KeyOperation)); value.AsString() != operation {
					continue
				}
				value, _ := point.Attributes.Value(attribute.Key(metrics.KeyErrorType))
				return value.AsString()
			}
		}
	}
	t.Fatalf("no %s point for %s", metrics.RequestDurationSeconds, operation)
	return ""
}
