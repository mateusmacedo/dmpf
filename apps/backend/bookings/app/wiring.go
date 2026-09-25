package app

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"slices"

	httpedge "github.com/mateusmacedo/dmpf/apps/backend/bookings/app/http"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	obsusecase "github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(ctx context.Context, cfg Config, out io.Writer) error {
	return boot.Boot(ctx, out, TelemetryOf(cfg), func(ctx context.Context, rt *otelboot.Runtime) error {
		return RunWith(ctx, cfg, rt, out)
	})
}

// RunWith is the delegate a caller that already owns the telemetry enters by,
// which is what the harnesses use.
func RunWith(ctx context.Context, cfg Config, rt *otelboot.Runtime, out io.Writer) error {
	switch cfg.Role {
	case RoleAPI:
		return serveAPI(ctx, cfg, rt, out)
	case RoleRelay:
		return runRelay(ctx, cfg, rt)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownRole, cfg.Role)
	}
}

func bindBookings(tx *postgres.Tx) application.Resources {
	return application.Resources{
		Bookings:  provider.NewBookingRepository(tx),
		Resources: provider.NewResourceRepository(tx),
		Outbox:    tx.Outbox(provider.Mapper{}),
	}
}

// NewBookingsService assembles the use cases over Postgres, which is the one
// place the concrete providers of this context are instantiated (ADR-015),
// with the instrumentation of FND-08 and the audit trail written to auditOut.
func NewBookingsService(pool *pgxpool.Pool, rt *otelboot.Runtime, cfg Config, auditOut io.Writer) application.Service {
	return application.Service{
		UoW:            postgres.NewUnitOfWork(pool, bindBookings),
		Reader:         provider.NewBookingReader(postgres.NewReadPool(pool)),
		ResourceReader: provider.NewBookingsByResourceReader(postgres.NewReadPool(pool)),
		Clock:          idclock.SystemClock{},
		IDs:            idclock.NewMessageIDs("bookings"),
		Authorize:      Authorization(),
		Instrumentation: obsusecase.New(rt,
			audit.NewEnvelopeSink(auditOut, audit.Identity{Service: cfg.Service, Version: cfg.Version, Instance: cfg.Instance}),
			carrierSubject, nil, application.OperationFindBooking, application.OperationFindByResource),
	}
}

func NewMux(service application.Service, budget deadline.Budget, authenticator ports.Authenticator) (*http.ServeMux, error) {
	return httpedge.Mux(service, budget, authenticator)
}

// Authenticator resolves how this process verifies identity. The development
// mock is only reachable through the opt-out the config already refused to
// combine with an issuer, so one start never has two ways of resolving it.
func Authenticator(ctx context.Context, cfg Config) (ports.Authenticator, error) {
	if cfg.Auth.DevMock {
		return authn.DevAuthenticator{}, nil
	}
	return authn.NewVerifier(ctx, cfg.Auth)
}

func serveAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime, out io.Writer) error {
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer())
	if err != nil {
		return err
	}
	defer pool.Close()
	if cfg.Migrate {
		if err := postgres.Migrate(ctx, pool, []postgres.Capability{postgres.Outbox}, provider.Schema); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}

	authenticator, err := Authenticator(ctx, cfg)
	if err != nil {
		return fmt.Errorf("authenticator: %w", err)
	}
	service := NewBookingsService(pool, rt, cfg, out)
	mux, err := NewMux(service, cfg.RouteBudget, authenticator)
	if err != nil {
		return fmt.Errorf("routes: %w", err)
	}
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: mux}
	failed := make(chan error, 2)
	if cfg.GRPCAddr != "" {
		grpcServe, err := grpcEdge(cfg, rt, pool, service)
		if err != nil {
			return fmt.Errorf("grpc: %w", err)
		}
		go func() { failed <- grpcServe(ctx) }()
	}
	go func() { failed <- server.ListenAndServe() }()
	rt.Logger().InfoContext(ctx, "http listening", "addr", cfg.HTTPAddr)

	select {
	case <-ctx.Done():
		return drain(server, rt)
	case err := <-failed:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// grpcEdge builds the gRPC server of the api over the same service the HTTP
// edge uses, with the kernel's server chain.
func grpcEdge(cfg Config, rt *otelboot.Runtime, pool *pgxpool.Pool, service application.Service) (func(context.Context) error, error) {
	ctrl, err := admission.NewController(kernel.MethodLimits(rpc.ServiceName, rpc.Methods(), cfg.Admission), cfg.MetricTenants, admission.DefaultMaxKeys)
	if err != nil {
		return nil, err
	}
	serverConfig, err := kernel.APIServerConfig(kernel.APIServer{
		CertFile:       cfg.GRPCCertFile,
		KeyFile:        cfg.GRPCKeyFile,
		ClientCAFile:   cfg.GRPCClientCAFile,
		TrustedClients: cfg.GRPCTrustedClients,
		Insecure:       cfg.GRPCInsecure,
		Services:       kernel.HealthServices(rpc.ServiceName),
		Interceptors:   kernel.ServerInterceptors(rpc.ServiceName, rt.Tracer(), ctrl, rt.Instruments(), rt.Logger()),
		Logger:         rt.Logger(),
	})
	if err != nil {
		return nil, err
	}
	server, healthServer, err := kernel.NewServer(serverConfig)
	if err != nil {
		return nil, err
	}
	server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: service})
	listen := func() (net.Listener, error) { return net.Listen("tcp", cfg.GRPCAddr) }
	ready := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		return nil
	}
	return func(ctx context.Context) error {
		return kernel.Serve(ctx, listen, server, healthServer, kernel.HealthServices(rpc.ServiceName), ready, rt.Logger())
	}, nil
}

// drain stops accepting and lets the requests in flight finish, because a
// forced close cuts answers the caller is waiting for.
func drain(server *http.Server, rt *otelboot.Runtime) error {
	grace, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), observability.ShutdownGrace)
	defer cancel()
	if err := server.Shutdown(grace); err != nil {
		rt.Logger().WarnContext(grace, "http shutdown grace expired", "error", err.Error())
		return server.Close()
	}
	return nil
}

func runRelay(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer())
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}

	catalog, err := NewCatalog(cfg)
	if err != nil {
		return err
	}
	if err := postgres.AssertOwnOutbox(ctx, pool, slices.Collect(maps.Keys(catalog))); err != nil {
		return err
	}
	kafkaConfig, err := kafka.NewConfig(ctx, rt, catalog, cfg.Brokers, cfg.Service, cfg.KafkaInsecure, cfg.KafkaAuth)
	if err != nil {
		return err
	}
	publisher, err := kafka.NewPublisher(kafkaConfig, nil)
	if err != nil {
		return err
	}
	defer publisher.Close()

	drain, err := relay.NewOverPostgres(pool, publisher, "bookings", cfg.Relay)
	if err != nil {
		return err
	}
	rt.Logger().InfoContext(ctx, "relay draining", "channel", application.Destination, "topic", cfg.BookingsTopic)
	return drain.Run(ctx)
}
