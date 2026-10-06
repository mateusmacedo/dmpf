package app

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"reflect"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/provider"
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

const keyChannelName = "dmpf.channel.name"

func Run(ctx context.Context, cfg Config) error {
	return boot.Boot(ctx, TelemetryOf(cfg), func(ctx context.Context, rt *otelboot.Runtime) error {
		return RunWith(ctx, cfg, rt)
	})
}

// RunWith runs the role over a runtime the caller booted.
func RunWith(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	switch cfg.Role {
	case RoleAPI:
		return serveAPI(ctx, cfg, rt)
	case RoleRelay:
		return runRelay(ctx, cfg, rt)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownRole, cfg.Role)
	}
}

// Waits are the ceiling of IDM-07 for a command.
type Waits struct {
	Command time.Duration
}

func bind(waits Waits) func(*postgres.Tx) application.Resources {
	return func(tx *postgres.Tx) application.Resources {
		return application.Resources{
			Orders:   provider.NewOrderRepository(tx),
			Outbox:   tx.Outbox(provider.Mapper{}),
			Commands: tx.CommandInbox(application.CommandConsumer, waits.Command),
		}
	}
}

func NewService(pool *pgxpool.Pool, clock ports.Clock, ids ports.IDGenerator, waits Waits) application.Service {
	return application.Service{
		UoW:       postgres.NewUnitOfWork(pool, bind(waits)),
		Reader:    provider.NewOrderReader(postgres.NewReadPool(pool)),
		Clock:     clock,
		IDs:       ids,
		Authorize: Authorization(),
	}
}

// NewOrdersService assembles the orders use cases over Postgres, with the
// instrumentation of FND-08 and the audit trail emitted through rt.
func NewOrdersService(pool *pgxpool.Pool, rt *otelboot.Runtime, cfg Config) (application.Service, error) {
	policy, err := kernelapp.IdempotencyPolicy(cfg.Policies.IdempotencyWait, cfg.Policies.IdempotencyRetention)
	if err != nil {
		return application.Service{}, err
	}
	service := NewService(pool, idclock.SystemClock{}, idclock.NewMessageIDs("orders"), Waits{Command: cfg.Policies.IdempotencyWait})
	service.ItemLimit = cfg.ItemLimit
	service.Idempotency = policy
	service.Instrumentation = obsusecase.New(rt, audit.NewLogSink(rt.LoggerProvider()), subject, classify, application.OperationFindOrder)
	return service, nil
}

func serveAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	ctx, abort := context.WithCancelCause(ctx)
	defer abort(nil)
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer(), postgres.WithMeterProvider(rt.MeterProvider()))
	if err != nil {
		return err
	}
	defer pool.Close()
	stopPurge := func() error { return nil }
	defer func() { _ = stopPurge() }()

	ctrl, err := admission.NewController(kernelgrpc.MethodLimits(rpc.ServiceName, rpc.Methods(), cfg.Admission), cfg.MetricTenants, admission.DefaultMaxKeys)
	if err != nil {
		return err
	}
	serverConfig, err := kernelgrpc.APIServerConfig(kernelgrpc.APIServer{
		CertFile:       cfg.API.GRPCCertFile,
		KeyFile:        cfg.API.GRPCKeyFile,
		ClientCAFile:   cfg.API.GRPCClientCAFile,
		TrustedClients: cfg.API.GRPCTrustedClients,
		Insecure:       cfg.API.GRPCInsecure,
		Services:       kernelgrpc.HealthServices(rpc.ServiceName),
		Interceptors:   kernelgrpc.ServerInterceptors(rpc.ServiceName, ctrl, rt.Instruments(), rt.LoggerProvider(), kernelgrpc.WithCommands(rpc.Commands()...)),
		LoggerProvider: rt.LoggerProvider(),
	})
	if err != nil {
		return err
	}
	server, healthServer, err := kernelgrpc.NewServer(serverConfig)
	if err != nil {
		return err
	}
	service, err := NewOrdersService(pool, rt, cfg)
	if err != nil {
		return err
	}
	server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: service})

	listen := func() (net.Listener, error) { return net.Listen("tcp", cfg.API.GRPCAddr) }
	ready := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		if cfg.Migrate {
			if err := postgres.Migrate(ctx, pool, []postgres.Capability{postgres.Outbox, postgres.Inbox}, provider.Schema); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
		}
		stop, err := kernelapp.StartPurge(ctx, abort,
			kernelapp.PurgeConfig{Name: "command-inbox", Interval: cfg.Policies.PurgeInterval, Batch: cfg.Policies.PurgeBatch},
			idclock.SystemClock{}, rt.LoggerProvider(), kernelapp.PurgeCommandInbox(pool, application.CommandConsumer))
		if err != nil {
			return err
		}
		stopPurge = stop
		return nil
	}
	if err := kernelgrpc.Serve(ctx, listen, server, healthServer, kernelgrpc.HealthServices(rpc.ServiceName), ready, rt.LoggerProvider()); err != nil {
		return err
	}
	return stopPurge()
}

func runRelay(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	ctx, abort := context.WithCancelCause(ctx)
	defer abort(nil)
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer(), postgres.WithMeterProvider(rt.MeterProvider()))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := postgres.WaitForTables(ctx, pool, time.Second, rt.LoggerProvider(), postgres.Tables(postgres.Outbox)...); err != nil {
		return err
	}

	catalog, err := NewCatalog(cfg)
	if err != nil {
		return err
	}
	if err := postgres.AssertOwnOutbox(ctx, pool, slices.Collect(maps.Keys(catalog))); err != nil {
		return err
	}
	stopPurge, err := kernelapp.StartPurge(ctx, abort,
		kernelapp.PurgeConfig{Name: "outbox", Interval: cfg.Policies.PurgeInterval, Batch: cfg.Policies.PurgeBatch, Retention: cfg.Policies.OutboxRetention},
		idclock.SystemClock{}, rt.LoggerProvider(), kernelapp.PurgeOutbox(pool))
	if err != nil {
		return err
	}
	defer func() { _ = stopPurge() }()

	kafkaConfig, err := kafka.NewConfig(ctx, rt, catalog, cfg.Brokers, cfg.KafkaInsecure, cfg.KafkaAuth)
	if err != nil {
		return err
	}
	publisher, err := kafka.NewPublisher(kafkaConfig, nil)
	if err != nil {
		return err
	}
	defer publisher.Close()

	drain, err := relay.NewOverPostgres(pool, publisher, "orders", relay.Instrument(cfg.Relay, rt, catalog.AddressOf))
	if err != nil {
		return err
	}
	rt.LoggerFor(reflect.TypeFor[Config]().PkgPath()).LogAttrs(ctx, slog.LevelInfo, "relay draining",
		slog.String(string(semconv.MessagingSystemKey), semconv.MessagingSystemKafka.Value.AsString()),
		slog.String(string(semconv.MessagingDestinationNameKey), cfg.OrdersTopic),
		slog.String(keyChannelName, application.Destination))
	if err := drain.Run(ctx); err != nil {
		return err
	}
	return stopPurge()
}
