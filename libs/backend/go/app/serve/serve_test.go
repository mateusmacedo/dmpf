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
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

const unreachable = "postgres://dmpf@127.0.0.1:1/serve?connect_timeout=1"

var logs = &obstest.MemoryLogs{}

var started = sync.OnceValues(func() (*otelboot.Runtime, error) {
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "serve", ServiceVersion: "dev", ServiceInstanceID: "serve-1"},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)
	return otelboot.Start(context.Background(), config)
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

func TestServeAPIRefusesARegisterOfAnotherService(t *testing.T) {
	err := serve.ServeAPI(context.Background(), runtime(t),
		serve.APIRole{
			DSN:       unreachable,
			Server:    kernelgrpc.APIEnv{GRPCAddr: "127.0.0.1:0", GRPCInsecure: true},
			Admission: admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 1},
		},
		serve.API{
			Desc: &grpc.ServiceDesc{ServiceName: "dmpf.serve.v1.ProbeService", Methods: []grpc.MethodDesc{{MethodName: "Probe"}}},
			Register: func(server *grpc.Server, _ *pgxpool.Pool) error {
				server.RegisterService(&grpc.ServiceDesc{ServiceName: "dmpf.serve.v1.OtherService", HandlerType: (*any)(nil)}, struct{}{})
				return nil
			},
		})

	if !errors.Is(err, serve.ErrServiceNotRegistered) {
		t.Fatalf("ServeAPI() = %v, want %v: the admission and the chain were built for ProbeService", err, serve.ErrServiceNotRegistered)
	}
}

func TestRunRelayRefusesARelayWithoutComponentOrScope(t *testing.T) {
	ch := channel.NewKafka("serve.events", "serve", "serve.dlq", "serve-relay", "com.company.serve.probed", "serve.v1")
	for name, r := range map[string]serve.Relay{
		"component": {Channel: ch, Scope: "serve"},
		"scope":     {Component: "serve", Channel: ch},
	} {
		t.Run(name, func(t *testing.T) {
			if err := serve.RunRelay(context.Background(), runtime(t), serve.RelayRole{DSN: unreachable}, r); !errors.Is(err, serve.ErrRelayUnnamed) {
				t.Fatalf("RunRelay() = %v, want %v before opening Postgres", err, serve.ErrRelayUnnamed)
			}
		})
	}
}

func TestRunRelayStopsWhenPostgresIsUnreachable(t *testing.T) {
	err := serve.RunRelay(context.Background(), runtime(t),
		serve.RelayRole{DSN: unreachable},
		serve.Relay{
			Component: "serve",
			Channel:   channel.NewKafka("serve.events", "serve", "serve.dlq", "serve-relay", "com.company.serve.probed", "serve.v1"),
			Scope:     "serve",
		})

	if err == nil || !strings.HasPrefix(err.Error(), "postgres: ") {
		t.Fatalf("RunRelay() = %v, want the postgres ping failure", err)
	}
}

const probe = "dmpf.serve.v1.ProbeService"

func probeDesc() *grpc.ServiceDesc {
	return &grpc.ServiceDesc{ServiceName: probe, HandlerType: (*any)(nil), Methods: []grpc.MethodDesc{{MethodName: "Probe"}}}
}

func registerProbe(server *grpc.Server, _ *pgxpool.Pool) error {
	server.RegisterService(probeDesc(), struct{}{})
	return nil
}

func apiRole() serve.APIRole {
	return serve.APIRole{
		DSN:       unreachable,
		Server:    kernelgrpc.APIEnv{GRPCAddr: "127.0.0.1:0", GRPCInsecure: true},
		Admission: admission.Limit{PerSecond: 1, Burst: 1, Concurrency: 1},
	}
}

func TestServeAPIRefusesAnIncompleteAPIBeforeOpeningPostgres(t *testing.T) {
	streaming := probeDesc()
	streaming.Streams = []grpc.StreamDesc{{StreamName: "Watch"}}
	for name, c := range map[string]struct {
		rt   *otelboot.Runtime
		api  serve.API
		want error
	}{
		"runtime":  {nil, serve.API{Desc: probeDesc(), Register: registerProbe}, serve.ErrNoRuntime},
		"desc":     {runtime(t), serve.API{Register: registerProbe}, serve.ErrAPIIncomplete},
		"register": {runtime(t), serve.API{Desc: probeDesc()}, serve.ErrAPIIncomplete},
		"methods":  {runtime(t), serve.API{Desc: &grpc.ServiceDesc{ServiceName: probe}, Register: registerProbe}, serve.ErrNoMethods},
		"streams":  {runtime(t), serve.API{Desc: streaming, Register: registerProbe}, serve.ErrStreamsNotServed},
		"command":  {runtime(t), serve.API{Desc: probeDesc(), Commands: []string{"Place"}, Register: registerProbe}, serve.ErrUnknownCommand},
		"platform": {runtime(t), serve.API{Desc: probeDesc(), Platform: []string{"Place"}, Register: registerProbe}, serve.ErrUnknownPlatformMethod},
	} {
		t.Run(name, func(t *testing.T) {
			if err := serve.ServeAPI(context.Background(), c.rt, apiRole(), c.api); !errors.Is(err, c.want) {
				t.Fatalf("ServeAPI() = %v, want %v", err, c.want)
			}
		})
	}
}

func TestServeAPIRefusesAPlatformMethodOnAServerThatDoesNotVerifyItsCallers(t *testing.T) {
	verified := kernelgrpc.APIEnv{GRPCAddr: "127.0.0.1:0", GRPCCertFile: "absent.crt", GRPCKeyFile: "absent.key", GRPCClientCAFile: "absent.ca", GRPCTrustedClients: []string{"spiffe://dmpf/bff"}}
	insecure := verified
	insecure.GRPCInsecure = true
	plaintext := kernelgrpc.APIEnv{GRPCAddr: "127.0.0.1:0", GRPCTrustedClients: verified.GRPCTrustedClients}
	untrusted := verified
	untrusted.GRPCTrustedClients = nil
	for name, server := range map[string]kernelgrpc.APIEnv{
		"insecure":          apiRole().Server,
		"insecure with tls": insecure,
		"without tls":       plaintext,
		"without trusted":   untrusted,
	} {
		t.Run(name, func(t *testing.T) {
			role := apiRole()
			role.Server = server

			err := serve.ServeAPI(context.Background(), runtime(t), role, serve.API{Desc: probeDesc(), Platform: []string{"Probe"}, Register: registerProbe})

			if !errors.Is(err, serve.ErrPlatformUnverified) {
				t.Fatalf("ServeAPI() = %v, want %v", err, serve.ErrPlatformUnverified)
			}
		})
	}
}

func TestServeAPIAdmitsAPlatformMethodOnAServerThatVerifiesItsCallers(t *testing.T) {
	role := apiRole()
	role.Server = kernelgrpc.APIEnv{GRPCAddr: "127.0.0.1:0", GRPCCertFile: "absent.crt", GRPCKeyFile: "absent.key", GRPCClientCAFile: "absent.ca", GRPCTrustedClients: []string{"spiffe://dmpf/bff"}}

	err := serve.ServeAPI(context.Background(), runtime(t), role, serve.API{Desc: probeDesc(), Platform: []string{"Probe"}, Register: registerProbe})

	if err == nil || errors.Is(err, serve.ErrPlatformUnverified) || errors.Is(err, serve.ErrUnknownPlatformMethod) {
		t.Fatalf("ServeAPI() = %v, want the failure to load the absent certificate, past the platform check", err)
	}
}

func TestServeAPIRefusesAServiceBesidesTheOneOfDesc(t *testing.T) {
	err := serve.ServeAPI(context.Background(), runtime(t), apiRole(), serve.API{
		Desc: probeDesc(),
		Register: func(server *grpc.Server, pool *pgxpool.Pool) error {
			server.RegisterService(&grpc.ServiceDesc{ServiceName: "dmpf.serve.v1.AdminService", HandlerType: (*any)(nil)}, struct{}{})
			return registerProbe(server, pool)
		},
	})

	if !errors.Is(err, serve.ErrUnexpectedService) {
		t.Fatalf("ServeAPI() = %v, want %v: the chain lets any other service through without admission", err, serve.ErrUnexpectedService)
	}
}

func TestServeAPIEndsCleanlyWhenStoppedBeforeServing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := serve.ServeAPI(ctx, runtime(t), apiRole(), serve.API{Desc: probeDesc(), Register: registerProbe}); err != nil {
		t.Fatalf("ServeAPI() = %v, want nil: a stop requested before the port listens is not a failure", err)
	}
}

func TestRunRelayRefusesAnInvalidChannelBeforeOpeningPostgres(t *testing.T) {
	err := serve.RunRelay(context.Background(), runtime(t), serve.RelayRole{DSN: unreachable}, serve.Relay{Component: "serve", Scope: "serve"})

	if err == nil || strings.HasPrefix(err.Error(), "postgres: ") {
		t.Fatalf("RunRelay() = %v, want the refusal of the channel before any Postgres call", err)
	}
}

func TestRunRelayRefusesANilRuntime(t *testing.T) {
	if err := serve.RunRelay(context.Background(), nil, serve.RelayRole{DSN: unreachable}, serve.Relay{}); !errors.Is(err, serve.ErrNoRuntime) {
		t.Fatalf("RunRelay() = %v, want %v", err, serve.ErrNoRuntime)
	}
}

func TestRunRelayEndsCleanlyWhenStoppedBeforeDraining(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch := channel.NewKafka("serve.events", "serve", "serve.dlq", "serve-relay", "com.company.serve.probed", "serve.v1")

	if err := serve.RunRelay(ctx, runtime(t), serve.RelayRole{DSN: unreachable}, serve.Relay{Component: "serve", Channel: ch, Scope: "serve"}); err != nil {
		t.Fatalf("RunRelay() = %v, want nil: a stop requested before the drain is not a failure", err)
	}
}
