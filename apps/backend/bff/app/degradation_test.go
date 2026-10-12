package app_test

import (
	"context"
	"encoding/json"
	"maps"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	ordersv1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/service/v1"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

const (
	requestLimit = 2 * time.Second
	drainLimit   = 5 * time.Second

	degradedCredential = `Bearer {"sub":"tester","tenant":"acme","permissions":["orders:read"]}`
)

type ordersContext struct {
	target string
	calls  atomic.Int64
}

var ordersService = ordersv1.File_company_orders_service_v1_orders_service_proto.Services().ByName("OrdersService")

func serveOrders(t *testing.T) *ordersContext {
	t.Helper()
	return serveOrdersHeldBy(t, func(context.Context) {})
}

func serveOrdersHeldBy(t *testing.T, hold func(context.Context)) *ordersContext {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	orders := &ordersContext{target: "dns:///" + listener.Addr().String()}
	server := grpc.NewServer()
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: rpc.OrdersServiceName,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			kernelgrpc.Unary(rpc.OrdersServiceName, kernelgrpc.Method(ordersService, "FindOrder"),
				func(_ any, ctx context.Context, req *ordersv1.FindOrderRequest) (*ordersv1.FindOrderResponse, error) {
					orders.calls.Add(1)
					hold(ctx)
					return &ordersv1.FindOrderResponse{Order: &ordersv1.Order{
						OrderId: req.GetOrderId(), Status: ordersv1.OrderStatus_ORDER_STATUS_OPEN, ItemLimit: 10,
					}}, nil
				}),
		},
	}, struct{}{})
	healthServer := health.NewServer()
	for _, service := range []string{rpc.OrdersServiceName, rpc.ReservationsServiceName, rpc.BookingsServiceName} {
		healthServer.SetServingStatus(service, healthpb.HealthCheckResponse_SERVING)
	}
	healthpb.RegisterHealthServer(server, healthServer)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return orders
}

func startEdgeExportingTo(t *testing.T, collector string, orders *ordersContext) app.Config {
	t.Helper()
	env := maps.Clone(obstest.ReadEnvFile(t, manifestPath))
	maps.Copy(env, map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + collector,
		"OTEL_METRIC_EXPORT_INTERVAL": "10",
		"OTEL_BSP_SCHEDULE_DELAY":     "10",
		"OTEL_BSP_MAX_QUEUE_SIZE":     "8",
		"OTEL_BLRP_SCHEDULE_DELAY":    "10",
		"OTEL_BLRP_MAX_QUEUE_SIZE":    "8",
		"HTTP_ADDR":                   obstest.UnusedLoopbackAddr(t),
		"ADMIN_ADDR":                  obstest.UnusedLoopbackAddr(t),
		"ORDERS_GRPC_TARGET":          orders.target,
		"RESERVATIONS_GRPC_TARGET":    orders.target,
		"BOOKINGS_GRPC_TARGET":        orders.target,
		"OPENAPI_ORDERS_PATH":         "",
		"OPENAPI_RESERVATIONS_PATH":   "",
		"OPENAPI_BOOKINGS_PATH":       "",
	})
	obstest.UseOnlyTheOTelEnvironmentOf(t, env)
	cfg, err := app.FromEnv(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("FromEnv() over %s = %v", manifestPath, err)
	}

	runtime, err := boot.StartTelemetry(context.Background(), app.TelemetryOf(cfg))
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	t.Cleanup(func() {
		grace, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		obstest.ReturnsWithin(t, drainLimit, "the telemetry shutdown", func() { _ = runtime.Shutdown(grace) })
	})
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- app.RunWith(ctx, cfg, runtime) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-served:
			if err != nil {
				t.Errorf("RunWith() = %v, want nil on shutdown", err)
			}
		case <-time.After(drainLimit):
			t.Errorf("the drain did not return in %v with the export failing, want the telemetry never to hold it (NF Degradação)", drainLimit)
		}
	})

	var notReady error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		probing, stop := context.WithTimeout(context.Background(), requestLimit)
		notReady = app.Probe(probing, cfg)
		stop()
		if notReady == nil {
			return cfg
		}
	}
	t.Fatalf("Probe() = %v, want the edge ready against the orders context with the export failing", notReady)
	return cfg
}

func TestAFailingExportNeverFailsNorHoldsARequestOfTheEdge(t *testing.T) {
	for name, collector := range map[string]func(testing.TB) *obstest.DownCollector{
		"unreachable endpoint": obstest.UnreachableCollector,
		"stalled collector":    obstest.StallingCollector,
	} {
		t.Run(name, func(t *testing.T) {
			down := collector(t)
			orders := serveOrders(t)
			cfg := startEdgeExportingTo(t, down.Addr, orders)
			client := &http.Client{Timeout: requestLimit}
			t.Cleanup(client.CloseIdleConnections)

			requests := int64(cfg.Admission.Burst)
			for n := range requests {
				id := "o-" + strconv.FormatInt(n, 10)
				req, err := http.NewRequest(http.MethodGet, "http://"+cfg.HTTPAddr+"/orders/"+id, nil)
				if err != nil {
					t.Fatalf("NewRequest() = %v", err)
				}
				req.Header.Set("Authorization", degradedCredential)
				began := time.Now()
				res, err := client.Do(req)
				if err != nil {
					t.Fatalf("GET /orders/%s = %v after %v with the export failing, want 200 within %v: telemetry never fails nor holds a request (NF Degradação)",
						id, err, time.Since(began).Round(time.Millisecond), requestLimit)
				}
				var order struct {
					ID string `json:"id"`
				}
				decoded := json.NewDecoder(res.Body).Decode(&order)
				_ = res.Body.Close()
				if res.StatusCode != http.StatusOK || decoded != nil || order.ID != id {
					t.Fatalf("GET /orders/%s = %d with order %q (%v), want 200 with the order the context answered", id, res.StatusCode, order.ID, decoded)
				}
			}
			if calls := orders.calls.Load(); calls != requests {
				t.Fatalf("the orders context answered %d calls for %d requests, want every request through its handler", calls, requests)
			}
			down.RequireEverySignalExporting(t)
		})
	}
}
