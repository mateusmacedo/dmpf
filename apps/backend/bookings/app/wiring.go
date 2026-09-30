package app

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	obsusecase "github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
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

func bindBookings(wait time.Duration) func(*postgres.Tx) application.Resources {
	return func(tx *postgres.Tx) application.Resources {
		return application.Resources{
			Bookings:  provider.NewBookingRepository(tx),
			Resources: provider.NewResourceRepository(tx),
			Outbox:    tx.Outbox(provider.Mapper{}),
			Commands:  tx.CommandInbox(application.CommandConsumer, wait),
		}
	}
}

// NewBookingsService assembles the use cases over Postgres, which is the one
// place the concrete providers of this context are instantiated (ADR-015),
// with the instrumentation of FND-08 and the audit trail written to auditOut.
func NewBookingsService(pool *pgxpool.Pool, rt *otelboot.Runtime, cfg Config, auditOut io.Writer) (application.Service, error) {
	policy, err := kernelapp.IdempotencyPolicy(cfg.IdempotencyWait, cfg.IdempotencyRetention)
	if err != nil {
		return application.Service{}, err
	}
	return application.Service{
		UoW:            postgres.NewUnitOfWork(pool, bindBookings(cfg.IdempotencyWait)),
		Reader:         provider.NewBookingReader(postgres.NewReadPool(pool)),
		ResourceReader: provider.NewBookingsByResourceReader(postgres.NewReadPool(pool)),
		Clock:          idclock.SystemClock{},
		IDs:            idclock.NewMessageIDs("bookings"),
		Authorize:      Authorization(),
		Idempotency:    policy,
		Instrumentation: obsusecase.New(rt,
			audit.NewEnvelopeSink(auditOut, audit.Identity{Service: cfg.Service, Version: cfg.Version, Instance: cfg.Instance}),
			subject, classify, application.OperationFindBooking, application.OperationFindBookingByResource),
	}, nil
}

func startPurge(ctx context.Context, cfg Config, rt *otelboot.Runtime, name string, retention time.Duration, fn kernelapp.PurgeFunc) (func(), error) {
	return kernelapp.StartPurge(ctx, kernelapp.PurgeConfig{Name: name, Interval: cfg.PurgeInterval, Batch: cfg.PurgeBatch, Retention: retention},
		idclock.SystemClock{}, rt.Logger(), fn)
}

func serveAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime, out io.Writer) error {
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer())
	if err != nil {
		return err
	}
	defer pool.Close()
	stopPurge := func() {}
	defer func() { stopPurge() }()

	ctrl, err := admission.NewController(kernelgrpc.MethodLimits(rpc.ServiceName, rpc.Methods(), cfg.Admission), cfg.MetricTenants, admission.DefaultMaxKeys)
	if err != nil {
		return err
	}
	serverConfig, err := kernelgrpc.APIServerConfig(kernelgrpc.APIServer{
		CertFile:       cfg.GRPCCertFile,
		KeyFile:        cfg.GRPCKeyFile,
		ClientCAFile:   cfg.GRPCClientCAFile,
		TrustedClients: cfg.GRPCTrustedClients,
		Insecure:       cfg.GRPCInsecure,
		Services:       kernelgrpc.HealthServices(rpc.ServiceName),
		Interceptors:   kernelgrpc.ServerInterceptors(rpc.ServiceName, rt.Tracer(), ctrl, rt.Instruments(), rt.Logger(), kernelgrpc.WithCommands(rpc.Commands()...)),
		Logger:         rt.Logger(),
	})
	if err != nil {
		return err
	}
	server, healthServer, err := kernelgrpc.NewServer(serverConfig)
	if err != nil {
		return err
	}
	service, err := NewBookingsService(pool, rt, cfg, out)
	if err != nil {
		return err
	}
	server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: service})

	listen := func() (net.Listener, error) { return net.Listen("tcp", cfg.GRPCAddr) }
	ready := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		if cfg.Migrate {
			if err := postgres.Migrate(ctx, pool, []postgres.Capability{postgres.Outbox, postgres.Inbox}, provider.Schema); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
		}
		stop, err := startPurge(ctx, cfg, rt, "command-inbox", 0, func(ctx context.Context, cutoff ports.Instant, batch int) (int64, error) {
			purged, err := postgres.PurgeExpiredInbox(ctx, pool, application.CommandConsumer, cutoff, batch)
			return purged.Removed, err
		})
		if err != nil {
			return err
		}
		stopPurge = stop
		return nil
	}
	return kernelgrpc.Serve(ctx, listen, server, healthServer, kernelgrpc.HealthServices(rpc.ServiceName), ready, rt.Logger())
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
	if err := postgres.WaitForTables(ctx, pool, time.Second, rt.Logger(), postgres.Tables(postgres.Outbox)...); err != nil {
		return err
	}

	catalog, err := NewCatalog(cfg)
	if err != nil {
		return err
	}
	if err := postgres.AssertOwnOutbox(ctx, pool, slices.Collect(maps.Keys(catalog))); err != nil {
		return err
	}
	stopPurge, err := startPurge(ctx, cfg, rt, "outbox", cfg.OutboxRetention, func(ctx context.Context, cutoff ports.Instant, batch int) (int64, error) {
		purged, err := postgres.PurgePublished(ctx, pool, cutoff, batch)
		return purged.Count, err
	})
	if err != nil {
		return err
	}
	defer stopPurge()

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
