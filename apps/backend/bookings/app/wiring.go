package app

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net"
	"slices"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	obsusecase "github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"

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
			subject, classify, application.OperationFindBooking, application.OperationFindBookingByResource),
	}
}

func serveAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime, out io.Writer) error {
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer())
	if err != nil {
		return err
	}
	defer pool.Close()

	ctrl, err := admission.NewController(kernel.MethodLimits(rpc.ServiceName, rpc.Methods(), cfg.Admission), cfg.MetricTenants, admission.DefaultMaxKeys)
	if err != nil {
		return err
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
		return err
	}
	server, healthServer, err := kernel.NewServer(serverConfig)
	if err != nil {
		return err
	}
	server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: NewBookingsService(pool, rt, cfg, out)})

	listen := func() (net.Listener, error) { return net.Listen("tcp", cfg.GRPCAddr) }
	ready := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		if cfg.Migrate {
			if err := postgres.Migrate(ctx, pool, []postgres.Capability{postgres.Outbox}, provider.Schema); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
		}
		return nil
	}
	return kernel.Serve(ctx, listen, server, healthServer, kernel.HealthServices(rpc.ServiceName), ready, rt.Logger())
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
