package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

type absentOrder struct{}

func (absentOrder) Load(context.Context, domain.OrderID) (domain.Snapshot, ports.Version, error) {
	return domain.Snapshot{}, 0, ports.ErrNotFound
}

func TestAFindOfAnAbsentOrderIsNotFoundOnTheUseCaseSpanAndItsRED(t *testing.T) {
	ctx := context.Background()
	spans := tracetest.NewInMemoryExporter()
	reader := sdkmetric.NewManualReader()
	runtime, err := otelboot.Start(ctx, otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "orders", ServiceVersion: "dev", ServiceInstanceID: "orders-1"},
		TraceExporter: spans,
		MetricReader:  reader,
	})
	if err != nil {
		t.Fatalf("Start() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	pool, err := pgxpool.New(ctx, "postgres://orders@127.0.0.1:1/orders")
	if err != nil {
		t.Fatalf("pgxpool.New() = %v, want nil", err)
	}
	defer pool.Close()
	service, err := app.NewOrdersService(pool, runtime, app.Defaults(app.RoleAPI))
	if err != nil {
		t.Fatalf("NewOrdersService() = %v, want nil", err)
	}
	service.Reader = absentOrder{}

	if _, err := service.FindOrder(tenantScoped(t), "order-404"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("FindOrder() = %v, want %v", err, ports.ErrNotFound)
	}
	if err := runtime.ForceFlush(ctx); err != nil {
		t.Fatalf("ForceFlush() = %v, want nil", err)
	}

	want := string(usecase.NotFound)
	if got := useCaseSpanErrorType(t, spans, application.OperationFindOrder); got != want {
		t.Errorf("span error.type = %q, want %q: the edge answers NotFound for the same error", got, want)
	}
	if got := useCaseREDErrorType(t, reader, application.OperationFindOrder); got != want {
		t.Errorf("%s error.type = %q, want %q", metrics.RequestDurationSeconds, got, want)
	}
}

func tenantScoped(t *testing.T) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.WithoutSubject())
}

func useCaseSpanErrorType(t *testing.T, spans *tracetest.InMemoryExporter, operation string) string {
	t.Helper()
	for _, span := range spans.GetSpans() {
		if span.Name != "dmpf.usecase."+operation {
			continue
		}
		for _, kv := range span.Attributes {
			if string(kv.Key) == metrics.KeyErrorType {
				return kv.Value.AsString()
			}
		}
		t.Fatalf("span %s carries no %s: %v", span.Name, metrics.KeyErrorType, span.Attributes)
	}
	t.Fatalf("spans = %v, want dmpf.usecase.%s", spans.GetSpans().Snapshots(), operation)
	return ""
}

func useCaseREDErrorType(t *testing.T, reader *sdkmetric.ManualReader, operation string) string {
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
