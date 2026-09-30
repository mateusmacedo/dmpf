package grpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

func commandChain(t *testing.T) grpc.UnaryServerInterceptor {
	t.Helper()
	return loggingCommandChain(t, nil)
}

func loggingCommandChain(t *testing.T, logger *slog.Logger) grpc.UnaryServerInterceptor {
	t.Helper()
	ctrl, err := admission.New(admission.Config{
		Limits:  kernel.MethodLimits(chainService, []string{"Probe", "Find"}, generous),
		MaxKeys: 16,
		Clock:   clock.System(),
	})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	chain := kernel.ServerInterceptors(chainService, noop.NewTracerProvider().Tracer("command-test"), ctrl, nil, logger,
		kernel.WithCommands("Probe"))
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		next := handler
		for i := len(chain) - 1; i >= 0; i-- {
			interceptor, inner := chain[i], next
			next = func(ctx context.Context, req any) (any, error) { return interceptor(ctx, req, info, inner) }
		}
		return next(ctx, req)
	}
}

func TestTheChainRefusesACommandWithoutAValidKey(t *testing.T) {
	for reason, pairs := range map[string][]string{
		kernel.ReasonMissingIdempotencyKey: {kernel.TenantKey, "acme"},
		kernel.ReasonInvalidIdempotencyKey: {kernel.TenantKey, "acme", kernel.IdempotencyKey, "k 1"},
	} {
		t.Run(reason, func(t *testing.T) {
			called := false
			_, err := commandChain(t)(incoming(t, pairs...), nil, &grpc.UnaryServerInfo{FullMethod: chainMethod},
				func(context.Context, any) (any, error) { called = true; return nil, nil })

			if status.Code(err) != codes.InvalidArgument || kernel.ReasonOf(err) != reason || called {
				t.Fatalf("command = (%v, %q), handler called = %t; want InvalidArgument with %s before the handler", status.Code(err), kernel.ReasonOf(err), called, reason)
			}
		})
	}
}

func TestTheChainHandsTheKeyOfACommandToTheUseCase(t *testing.T) {
	var (
		key     string
		carried bool
	)
	_, err := commandChain(t)(incoming(t, kernel.TenantKey, "acme", kernel.IdempotencyKey, "k-1"), nil,
		&grpc.UnaryServerInfo{FullMethod: chainMethod},
		func(ctx context.Context, _ any) (any, error) {
			key, carried = ports.IdempotencyKeyFrom(ctx)
			ports.MarkIdempotency(ctx, ports.IdempotencyNew)
			if _, marked := ports.IdempotencyOutcomeFrom(ctx); !marked {
				t.Error("the chain installed no slot: a replay could never be reported")
			}
			return nil, nil
		})
	if err != nil {
		t.Fatalf("command = %v, want nil", err)
	}
	if !carried || key != "k-1" {
		t.Fatalf("IdempotencyKeyFrom = %q, %t; want k-1, true", key, carried)
	}
}

func TestTheChainLogsAKeyOutsideItsFormatOnlyAsInvalid(t *testing.T) {
	var logs bytes.Buffer
	chain := loggingCommandChain(t, slog.New(slog.NewJSONHandler(&logs, nil)))

	_, err := chain(incoming(t, kernel.TenantKey, "acme", kernel.IdempotencyKey, "k 1 forged=true"), nil,
		&grpc.UnaryServerInfo{FullMethod: "/" + chainService + "/Find"},
		func(context.Context, any) (any, error) { return nil, nil })
	if err != nil {
		t.Fatalf("read = %v, want nil", err)
	}

	var record map[string]any
	if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
		t.Fatalf("log = %q, want one JSON record: %v", logs.String(), err)
	}
	if _, logged := record["idempotency_key"]; logged || record["idempotency_key_invalid"] != true {
		t.Fatalf("log = %v, want idempotency_key_invalid and no idempotency_key: the metadata is caller input", record)
	}
}

func TestTheChainLeavesAReadWithoutKeyAlone(t *testing.T) {
	called := false
	_, err := commandChain(t)(incoming(t, kernel.TenantKey, "acme"), nil,
		&grpc.UnaryServerInfo{FullMethod: "/" + chainService + "/Find"},
		func(context.Context, any) (any, error) { called = true; return nil, nil })

	if err != nil || !called {
		t.Fatalf("read without key = %v, handler called = %t; want nil and called", err, called)
	}
}

type replayingHealth struct {
	healthpb.UnimplementedHealthServer
	outcome ports.IdempotencyOutcome
}

func (h replayingHealth) Check(ctx context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	ports.MarkIdempotency(ctx, h.outcome)
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func TestAReplayedCommandAnswersWithTheReplayHeader(t *testing.T) {
	const healthService = "grpc.health.v1.Health"
	for name, c := range map[string]struct {
		outcome ports.IdempotencyOutcome
		want    []string
	}{
		"replayed": {ports.IdempotencyReplayed, []string{"true"}},
		"new":      {ports.IdempotencyNew, nil},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl, err := admission.New(admission.Config{
				Limits:  kernel.MethodLimits(healthService, []string{"Check"}, generous),
				MaxKeys: 16,
				Clock:   clock.System(),
			})
			if err != nil {
				t.Fatalf("admission.New() = %v", err)
			}
			chain := kernel.ServerInterceptors(healthService, noop.NewTracerProvider().Tracer("replay-test"), ctrl, nil, nil,
				kernel.WithCommands("Check"))
			client := healthpb.NewHealthClient(connect(t, serve(t, replayingHealth{outcome: c.outcome}, grpc.ChainUnaryInterceptor(chain...))))

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ctx = metadata.AppendToOutgoingContext(ctx, kernel.TenantKey, "acme", kernel.IdempotencyKey, "k-1")
			var header metadata.MD
			if _, err := client.Check(ctx, &healthpb.HealthCheckRequest{}, grpc.Header(&header)); err != nil {
				t.Fatalf("Check() = %v, want nil", err)
			}
			if got := header.Get(kernel.ReplayedHeader); len(got) != len(c.want) || (len(got) == 1 && got[0] != c.want[0]) {
				t.Fatalf("%s = %v, want %v", kernel.ReplayedHeader, got, c.want)
			}
		})
	}
}
