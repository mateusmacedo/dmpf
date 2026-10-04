package app_test

import (
	"context"
	"encoding/json"
	"maps"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	ordersv1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/service/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
)

const (
	requestLimit = 2 * time.Second
	drainLimit   = 5 * time.Second

	degradedCredential = `Bearer {"sub":"tester","tenant":"acme","permissions":["orders:read"]}`
)

type downCollector struct {
	addr   string
	stalls bool
	held   atomic.Int64
}

func unusedLoopbackAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	return addr
}

func refusingCollector(t *testing.T) *downCollector {
	return &downCollector{addr: unusedLoopbackAddr(t)}
}

func stallingCollector(t *testing.T) *downCollector {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	collector := &downCollector{addr: listener.Addr().String(), stalls: true}
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, conn)
			mu.Unlock()
			collector.held.Add(1)
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range held {
			_ = conn.Close()
		}
	})
	return collector
}

func (c *downCollector) requireEverySignalExporting(t *testing.T) {
	t.Helper()
	if !c.stalls {
		return
	}
	for deadline := time.Now().Add(2 * time.Second); c.held.Load() < 3 && time.Now().Before(deadline); {
		time.Sleep(5 * time.Millisecond)
	}
	if held := c.held.Load(); held < 3 {
		t.Errorf("the stalled collector holds %d connections, want one per signal: traces, metrics and logs exporting to it", held)
	}
}

func returnsWithin(t *testing.T, limit time.Duration, what string, run func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		run()
	}()
	select {
	case <-done:
	case <-time.After(limit):
		t.Errorf("%s did not return in %v with the export failing, want the telemetry never to hold it (NF Degradação)", what, limit)
	}
}

type ordersContext struct {
	target string
	calls  atomic.Int64
}

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
		Methods: []grpc.MethodDesc{{
			MethodName: "FindOrder",
			Handler: func(_ any, ctx context.Context, decode func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				req := new(ordersv1.FindOrderRequest)
				if err := decode(req); err != nil {
					return nil, err
				}
				orders.calls.Add(1)
				hold(ctx)
				return &ordersv1.FindOrderResponse{Order: &ordersv1.Order{
					OrderId: req.GetOrderId(), Status: ordersv1.OrderStatus_ORDER_STATUS_OPEN, ItemLimit: 10,
				}}, nil
			},
		}},
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
	env := maps.Clone(readManifest(t))
	maps.Copy(env, map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + collector,
		"OTEL_METRIC_EXPORT_INTERVAL": "10",
		"OTEL_BSP_SCHEDULE_DELAY":     "10",
		"OTEL_BSP_MAX_QUEUE_SIZE":     "8",
		"OTEL_BLRP_SCHEDULE_DELAY":    "10",
		"OTEL_BLRP_MAX_QUEUE_SIZE":    "8",
		"HTTP_ADDR":                   unusedLoopbackAddr(t),
		"ADMIN_ADDR":                  unusedLoopbackAddr(t),
		"ORDERS_GRPC_TARGET":          orders.target,
		"RESERVATIONS_GRPC_TARGET":    orders.target,
		"BOOKINGS_GRPC_TARGET":        orders.target,
		"OPENAPI_ORDERS_PATH":         "",
		"OPENAPI_RESERVATIONS_PATH":   "",
		"OPENAPI_BOOKINGS_PATH":       "",
	})
	useOnlyTheOTelEnvironmentOf(t, env)
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
		returnsWithin(t, drainLimit, "the telemetry shutdown", func() { _ = runtime.Shutdown(grace) })
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
	for name, collector := range map[string]func(*testing.T) *downCollector{
		"unreachable endpoint": refusingCollector,
		"stalled collector":    stallingCollector,
	} {
		t.Run(name, func(t *testing.T) {
			down := collector(t)
			orders := serveOrders(t)
			cfg := startEdgeExportingTo(t, down.addr, orders)
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
			down.requireEverySignalExporting(t)
		})
	}
}
