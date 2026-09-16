package grpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

func TestServiceConfigDeclaresRoundRobinAndHealthCheck(t *testing.T) {
	var decoded struct {
		LoadBalancingConfig []map[string]any `json:"loadBalancingConfig"`
		HealthCheckConfig   struct {
			ServiceName string `json:"serviceName"`
		} `json:"healthCheckConfig"`
	}
	if err := json.Unmarshal([]byte(provider.ServiceConfig("orders.v1.Orders")), &decoded); err != nil {
		t.Fatalf("service config is not JSON: %v", err)
	}
	if len(decoded.LoadBalancingConfig) != 1 {
		t.Fatalf("loadBalancingConfig = %v, want one policy", decoded.LoadBalancingConfig)
	}
	if _, ok := decoded.LoadBalancingConfig[0]["round_robin"]; !ok {
		t.Fatalf("loadBalancingConfig = %v, want round_robin (GRP-12)", decoded.LoadBalancingConfig)
	}
	if decoded.HealthCheckConfig.ServiceName != "orders.v1.Orders" {
		t.Fatalf("healthCheckConfig.serviceName = %q (GRP-13)", decoded.HealthCheckConfig.ServiceName)
	}
}

func TestDialRefusesAnInvalidConfig(t *testing.T) {
	cfg := validConfig()
	cfg.TLS = nil
	if _, err := provider.Dial("passthrough:///bufnet", cfg); !errors.Is(err, provider.ErrTLSRequired) {
		t.Fatalf("Dial() = %v, want ErrTLSRequired", err)
	}
}

func TestDialAppliesDeadlineAndCompositionToTheCall(t *testing.T) {
	seen := make(chan deadlineSeen, 4)
	dialer := serve(t, &hop{name: "C", deadlines: seen})

	cfg := routeConfig(time.Second, 100*time.Millisecond)
	conn, err := provider.Dial("passthrough:///bufnet", cfg, grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Dial() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := healthpb.NewHealthClient(conn)

	t.Run("a context without deadline is refused before the wire", func(t *testing.T) {
		_, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
		if !errors.Is(err, deadline.ErrNoDeadline) {
			t.Fatalf("Check() = %v, want ErrNoDeadline (GRP-04)", err)
		}
		if len(seen) != 0 {
			t.Fatal("the server saw a call the client should have refused")
		}
	})

	t.Run("a bounded call reaches the server with a derived deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ctx = retry.WithBudget(ctx, retry.WithTotal(time.Second))
		callerDeadline, _ := ctx.Deadline()

		if _, err := client.Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
			t.Fatalf("Check() = %v, want nil", err)
		}
		hops := collect(t, seen, 1)
		if !hops["C"].ok || hops["C"].deadline >= callerDeadline.UnixNano() {
			t.Fatalf("server deadline %d is not below the caller's %d", hops["C"].deadline, callerDeadline.UnixNano())
		}
	})

	t.Run("an undeclared method is refused", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err := client.Watch(ctx, &healthpb.HealthCheckRequest{})
		if !errors.Is(err, provider.ErrMethodNotDeclared) {
			t.Fatalf("Watch() = %v, want ErrMethodNotDeclared (GRP-16)", err)
		}
	})
}

func TestNewServerReportsHealthPerService(t *testing.T) {
	server, healthServer, err := provider.NewServer(provider.ServerConfig{
		InsecureForDevelopmentOnly: true,
		Services:                   []string{"orders.v1.Orders", "payments.v1.Payments"},
	})
	if err != nil {
		t.Fatalf("NewServer() = %v, want nil", err)
	}
	dialer := listen(t, server)
	conn := connect(t, dialer)
	client := healthpb.NewHealthClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	check := func(service string) healthpb.HealthCheckResponse_ServingStatus {
		resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
		if err != nil {
			t.Fatalf("Check(%q) = %v, want nil", service, err)
		}
		return resp.GetStatus()
	}

	if got := check("orders.v1.Orders"); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("declared service starts as %v, want NOT_SERVING", got)
	}
	healthServer.SetServingStatus("orders.v1.Orders", healthpb.HealthCheckResponse_SERVING)
	if got := check("orders.v1.Orders"); got != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("after SetServingStatus = %v, want SERVING", got)
	}
	if got := check("payments.v1.Payments"); got != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("the other service = %v, want NOT_SERVING: health is per service (GRP-13)", got)
	}
	if _, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: "unknown.v1.Unknown"}); status.Code(err) != codes.NotFound {
		t.Fatalf("Check(unknown) = %v, want NotFound", err)
	}
}

func TestNewServerRefusesToRunWithoutTLSOrOptOut(t *testing.T) {
	if _, _, err := provider.NewServer(provider.ServerConfig{}); !errors.Is(err, provider.ErrTLSRequired) {
		t.Fatalf("NewServer() = %v, want ErrTLSRequired (GRP-15)", err)
	}
}

func TestNewServerAppliesTheUnaryInterceptors(t *testing.T) {
	calls := 0
	counting := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		calls++
		return handler(ctx, req)
	}
	server, _, err := provider.NewServer(provider.ServerConfig{
		InsecureForDevelopmentOnly: true,
		UnaryInterceptors:          []grpc.UnaryServerInterceptor{counting},
	})
	if err != nil {
		t.Fatalf("NewServer() = %v, want nil", err)
	}
	client := healthpb.NewHealthClient(connect(t, listen(t, server)))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}
	if calls != 1 {
		t.Fatalf("the interceptor ran %d times, want 1", calls)
	}
}
