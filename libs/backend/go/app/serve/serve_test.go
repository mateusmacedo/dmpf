package serve_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app/serve"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

const unreachable = "postgres://dmpf@127.0.0.1:1/serve?connect_timeout=1"

var started = sync.OnceValues(func() (*otelboot.Runtime, error) {
	return otelboot.Start(context.Background(), otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "serve", ServiceVersion: "dev", ServiceInstanceID: "serve-1"},
		TraceExporter: tracetest.NewInMemoryExporter(),
	})
})

func runtime(t *testing.T) *otelboot.Runtime {
	t.Helper()
	rt, err := started()
	if err != nil {
		t.Fatalf("otelboot.Start() = %v", err)
	}
	return rt
}

func TestServeAPIReturnsTheRegistrationFailureWithoutServing(t *testing.T) {
	refused := errors.New("service assembly refused")

	err := serve.ServeAPI(context.Background(), runtime(t),
		serve.APIRole{
			DSN:       unreachable,
			Server:    kernelgrpc.APIEnv{GRPCAddr: "127.0.0.1:0", GRPCInsecure: true},
			Admission: admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 1},
		},
		serve.API{
			Desc:     &grpc.ServiceDesc{ServiceName: "dmpf.serve.v1.ProbeService", Methods: []grpc.MethodDesc{{MethodName: "Probe"}}},
			Register: func(*grpc.Server, *pgxpool.Pool) error { return refused },
		})

	if !errors.Is(err, refused) {
		t.Fatalf("ServeAPI() = %v, want the registration failure", err)
	}
}

func TestRunRelayStopsWhenPostgresIsUnreachable(t *testing.T) {
	err := serve.RunRelay(context.Background(), runtime(t),
		serve.RelayRole{DSN: unreachable},
		serve.Relay{
			Component: "serve",
			Channel:   channel.NewKafka("serve.events", "serve", "serve.dlq", "serve-relay", "com.company.serve.probed", "serve.v1"),
		})

	if err == nil || !strings.HasPrefix(err.Error(), "postgres: ") {
		t.Fatalf("RunRelay() = %v, want the postgres ping failure", err)
	}
}
