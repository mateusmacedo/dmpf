package postgres_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func TestNewPoolRefusesAMalformedDSN(t *testing.T) {
	pool, err := postgres.NewPool(context.Background(), "://not-a-dsn", noop.NewTracerProvider().Tracer("test"))

	if err == nil {
		pool.Close()
		t.Fatal("NewPool() accepted a malformed DSN; the configuration is validated at startup")
	}
}

const (
	unreachableDSN  = "postgres://orders@db.example:6543/orders?sslmode=disable&pool_max_conns=7"
	unreachablePool = "db.example:6543/orders"
)

func collect(t *testing.T, reader *sdkmetric.ManualReader) map[string]metricdata.Aggregation {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	got := map[string]metricdata.Aggregation{}
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			got[m.Name] = m.Data
		}
	}
	return got
}

func sumPoint(t *testing.T, data metricdata.Aggregation, attrs ...attribute.KeyValue) int64 {
	t.Helper()
	sum, ok := data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("aggregation = %T, want metricdata.Sum[int64]", data)
	}
	want := attribute.NewSet(attrs...)
	for _, point := range sum.DataPoints {
		if point.Attributes.Equals(&want) {
			return point.Value
		}
	}
	t.Fatalf("no data point with %v in %+v", want.ToSlice(), sum.DataPoints)
	return 0
}

func poolName(name string) attribute.KeyValue {
	return attribute.String("db.client.connection.pool.name", name)
}

func connectionState(state string) attribute.KeyValue {
	return attribute.String("db.client.connection.state", state)
}

func TestTheMeteredPoolReportsTheConnectionInstrumentsOfDbconv(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	pool, err := postgres.NewPool(context.Background(), unreachableDSN, noop.NewTracerProvider().Tracer("test"),
		postgres.WithMeterProvider(provider))
	if err != nil {
		t.Fatalf("NewPool() = %v, want nil", err)
	}
	t.Cleanup(pool.Close)

	got := collect(t, reader)
	name := poolName(unreachablePool)
	if v := sumPoint(t, got["db.client.connection.count"], name, connectionState("idle")); v != 0 {
		t.Errorf("count{idle} = %d, want 0", v)
	}
	if v := sumPoint(t, got["db.client.connection.count"], name, connectionState("used")); v != 0 {
		t.Errorf("count{used} = %d, want 0", v)
	}
	if v := sumPoint(t, got["db.client.connection.max"], name); v != 7 {
		t.Errorf("max = %d, want the declared pool_max_conns 7", v)
	}
	if v := sumPoint(t, got["db.client.connection.pending_requests"], name); v != 0 {
		t.Errorf("pending_requests = %d, want 0", v)
	}
	if v := sumPoint(t, got["db.client.connection.timeouts"], name); v != 0 {
		t.Errorf("timeouts = %d, want 0", v)
	}
}

func TestAPoolWithoutAMeterProviderLeavesTheGlobalOneAlone(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	global := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(global)
	t.Cleanup(func() {
		otel.SetMeterProvider(previous)
		_ = global.Shutdown(context.Background())
	})

	pool, err := postgres.NewPool(context.Background(), unreachableDSN, noop.NewTracerProvider().Tracer("test"))
	if err != nil {
		t.Fatalf("NewPool() = %v, want nil", err)
	}
	t.Cleanup(pool.Close)

	if got := collect(t, reader); len(got) != 0 {
		t.Fatalf("the global provider received %v, want nothing: the pool takes its provider explicitly", got)
	}
}
