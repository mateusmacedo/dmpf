package dmpfgrpc_test

import (
	"context"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	dmpfgrpc "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-grpc"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/admission"
)

type tenantKey struct{}

func tenantFromContext(ctx context.Context) string {
	tenant, _ := ctx.Value(tenantKey{}).(string)
	return tenant
}

func withTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenant)
}

func admissionController(t *testing.T, limit admission.Limit) *admission.Controller {
	t.Helper()
	tenants, err := metrics.DeclareTenants("acme")
	if err != nil {
		t.Fatal(err)
	}
	ctrl, err := admission.New(admission.Config{
		Limits:  map[string]admission.Limit{checkMethod: limit},
		Tenants: tenants,
		MaxKeys: 10,
		Clock:   clock.NewFake(start),
	})
	if err != nil {
		t.Fatal(err)
	}
	return ctrl
}

func TestAdmissionRefusesBeforeTheHandlerAndCountsByRouteAndTenant(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	instruments, err := metrics.New(mp.Meter("test"))
	if err != nil {
		t.Fatal(err)
	}

	ctrl := admissionController(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 10})
	interceptor := dmpfgrpc.Admission(ctrl, tenantFromContext, instruments)
	info := &grpc.UnaryServerInfo{FullMethod: checkMethod}

	handled := 0
	handler := func(context.Context, any) (any, error) { handled++; return "ok", nil }

	// The burst admits one call from an undeclared tenant; the second is refused.
	if _, err := interceptor(withTenant(context.Background(), "initech"), nil, info, handler); err != nil {
		t.Fatalf("first call = %v, want nil", err)
	}
	_, err = interceptor(withTenant(context.Background(), "umbrella"), nil, info, handler)
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("second call = %v, want RESOURCE_EXHAUSTED (RES-17)", err)
	}
	if handled != 1 {
		t.Fatalf("handler ran %d times, want 1: the refusal comes before the handler", handled)
	}

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != metrics.AdmissionRejectionsTotal {
				continue
			}
			found = true
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok || len(sum.DataPoints) != 1 {
				t.Fatalf("series has %d data points, want 1", len(sum.DataPoints))
			}
			dp := sum.DataPoints[0]
			route, _ := dp.Attributes.Value(metrics.KeyRoute)
			tenant, _ := dp.Attributes.Value(metrics.KeyTenant)
			if route.AsString() != checkMethod || tenant.AsString() != metrics.OtherTenant {
				t.Fatalf("labels = route=%q tenant=%q, want the method and %q (MET-07, MET-12)", route.AsString(), tenant.AsString(), metrics.OtherTenant)
			}
			if dp.Value != 1 {
				t.Fatalf("rejections = %d, want 1", dp.Value)
			}
		}
	}
	if !found {
		t.Fatalf("series %q was not recorded", metrics.AdmissionRejectionsTotal)
	}
}

func TestAdmissionKeepsDeclaredTenantsApart(t *testing.T) {
	ctrl := admissionController(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 10})
	interceptor := dmpfgrpc.Admission(ctrl, tenantFromContext, nil)
	info := &grpc.UnaryServerInfo{FullMethod: checkMethod}
	handler := func(context.Context, any) (any, error) { return "ok", nil }

	if _, err := interceptor(withTenant(context.Background(), "acme"), nil, info, handler); err != nil {
		t.Fatalf("acme = %v, want nil", err)
	}
	if _, err := interceptor(withTenant(context.Background(), "initech"), nil, info, handler); err != nil {
		t.Fatalf("initech = %v, want nil: an undeclared tenant has its own bucket under other", err)
	}
	if _, err := interceptor(withTenant(context.Background(), "acme"), nil, info, handler); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("acme again = %v, want RESOURCE_EXHAUSTED", err)
	}
}

func TestAdmissionReleasesTheConcurrencySlotAfterTheHandler(t *testing.T) {
	ctrl := admissionController(t, admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 1})
	interceptor := dmpfgrpc.Admission(ctrl, nil, nil)
	info := &grpc.UnaryServerInfo{FullMethod: checkMethod}
	handler := func(context.Context, any) (any, error) { return "ok", nil }

	for range 3 {
		if _, err := interceptor(context.Background(), nil, info, handler); err != nil {
			t.Fatalf("sequential call = %v, want nil: the slot is released when the handler returns", err)
		}
	}
}

func TestAdmissionRefusesAnUndeclaredRouteAsUnimplemented(t *testing.T) {
	ctrl := admissionController(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 1})
	interceptor := dmpfgrpc.Admission(ctrl, nil, nil)
	handled := false
	handler := func(context.Context, any) (any, error) { handled = true; return nil, nil }

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/orders.v1.Orders/Place"}, handler)
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("undeclared route = %v, want UNIMPLEMENTED (RES-16)", err)
	}
	if handled {
		t.Fatal("the handler ran for a route with no declared limit")
	}
}

func TestAdmissionRunsInsideNewServer(t *testing.T) {
	ctrl := admissionController(t, admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 10})
	server, healthServer, err := dmpfgrpc.NewServer(dmpfgrpc.ServerConfig{
		InsecureForDevelopmentOnly: true,
		UnaryInterceptors:          []grpc.UnaryServerInterceptor{dmpfgrpc.Admission(ctrl, nil, nil)},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = healthServer
	client := healthClient(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Check(ctx, nil); err != nil {
		t.Fatalf("first Check() = %v, want nil", err)
	}
	if _, err := client.Check(ctx, nil); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("second Check() = %v, want RESOURCE_EXHAUSTED over the wire", err)
	}
}
