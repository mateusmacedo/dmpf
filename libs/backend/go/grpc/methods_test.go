package grpc_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
)

var healthService = healthpb.File_grpc_health_v1_health_proto.Services().ByName("Health")

type healthChecker struct{ err error }

func (h healthChecker) Check(_ context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	if h.err != nil {
		return nil, h.err
	}
	if req.GetService() == "" {
		return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_NOT_SERVING}, nil
	}
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

var checkDesc = kernelgrpc.Unary(healthpb.Health_ServiceDesc.ServiceName, kernelgrpc.Method(healthService, "Check"), healthChecker.Check)

func decodeService(service string) func(any) error {
	return func(m any) error {
		m.(*healthpb.HealthCheckRequest).Service = service
		return nil
	}
}

func TestMethodPanicsForAnUndeclaredName(t *testing.T) {
	defer func() {
		if got := recover(); got != "grpc: grpc.health.v1.Health declares no method Absent" {
			t.Fatalf("recover() = %v, want the panic naming the service and the method", got)
		}
	}()
	kernelgrpc.Method(healthService, "Absent")
}

func TestMethodNamesFollowTheDescriptor(t *testing.T) {
	if got := kernelgrpc.MethodNames(healthService); !slices.Equal(got, []string{"Check", "List", "Watch"}) {
		t.Fatalf("MethodNames() = %v, want [Check List Watch]", got)
	}
	if got := kernelgrpc.FullMethod("a.B", "C"); got != "/a.B/C" {
		t.Fatalf("FullMethod() = %q, want /a.B/C", got)
	}
}

func TestUncoveredNamesEveryDeclaredMethodWithoutExactlyOneDesc(t *testing.T) {
	partial := &grpc.ServiceDesc{Methods: []grpc.MethodDesc{{MethodName: "Check"}}}
	if got := kernelgrpc.Uncovered(partial, healthService); !slices.Equal(got, []string{"List", "Watch"}) {
		t.Fatalf("Uncovered() = %v, want [List Watch]", got)
	}

	complete := &grpc.ServiceDesc{
		Methods: []grpc.MethodDesc{{MethodName: "Check"}, {MethodName: "List"}},
		Streams: []grpc.StreamDesc{{StreamName: "Watch"}},
	}
	if got := kernelgrpc.Uncovered(complete, healthService); !slices.Equal(got, []string{"Watch"}) {
		t.Fatalf("Uncovered() = %v, want [Watch]: a stream is no unary method", got)
	}

	twice := &grpc.ServiceDesc{Methods: []grpc.MethodDesc{{MethodName: "Check"}, {MethodName: "Check"}, {MethodName: "List"}}}
	if got := kernelgrpc.Uncovered(twice, healthService); !slices.Equal(got, []string{"Check", "Watch"}) {
		t.Fatalf("Uncovered() = %v, want [Check Watch]: a method registered twice is not covered", got)
	}
}

func TestUnaryDispatchesTheDecodedRequestToTheServer(t *testing.T) {
	if checkDesc.MethodName != "Check" {
		t.Fatalf("MethodName = %q, want Check", checkDesc.MethodName)
	}

	resp, err := checkDesc.Handler(healthChecker{}, context.Background(), decodeService("orders"), nil)

	if err != nil || resp.(*healthpb.HealthCheckResponse).GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("Handler() = (%v, %v), want SERVING", resp, err)
	}
}

func TestUnaryHandsTheCallToTheInterceptor(t *testing.T) {
	var seen *grpc.UnaryServerInfo
	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		seen = info
		return handler(ctx, req)
	}
	server := healthChecker{}

	resp, err := checkDesc.Handler(server, context.Background(), decodeService("orders"), interceptor)

	if err != nil || resp.(*healthpb.HealthCheckResponse).GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("Handler() = (%v, %v), want SERVING", resp, err)
	}
	if seen == nil || seen.FullMethod != "/grpc.health.v1.Health/Check" || seen.Server != server {
		t.Fatalf("info = %+v, want /grpc.health.v1.Health/Check on the server", seen)
	}
}

func TestUnaryReturnsAnUntypedNilWithTheError(t *testing.T) {
	boom := errors.New("boom")

	resp, err := checkDesc.Handler(healthChecker{err: boom}, context.Background(), decodeService("orders"), nil)

	if !errors.Is(err, boom) || resp != nil {
		t.Fatalf("Handler() = (%#v, %v), want (nil, boom)", resp, err)
	}
}

func TestUnaryStopsAtADecodeFailure(t *testing.T) {
	malformed := errors.New("malformed")

	resp, err := checkDesc.Handler(healthChecker{}, context.Background(), func(any) error { return malformed }, nil)

	if !errors.Is(err, malformed) || resp != nil {
		t.Fatalf("Handler() = (%#v, %v), want (nil, malformed)", resp, err)
	}
}
