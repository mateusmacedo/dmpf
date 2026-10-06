package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"reflect"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/provider"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

const (

	// processingDeadline must stay below rebalanceTimeout (KFK-19): a
	// revocation waits for the record in flight, never for the whole queue.
	processingDeadline = 5 * time.Second
	rebalanceTimeout   = 30 * time.Second
	queuePerPartition  = 32

	keyChannelName = "dmpf.channel.name"
)

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
	case RoleConsumer:
		return runConsumer(ctx, cfg, rt)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownRole, cfg.Role)
	}
}

// NewReservationsService assembles the synchronous reservations use cases over
// Postgres, with the resource set the consumer also binds (INB-07).
func NewReservationsService(pool *pgxpool.Pool, rt *otelboot.Runtime, cfg Config) (application.Service, error) {
	policy, err := kernelapp.IdempotencyPolicy(cfg.Policies.IdempotencyWait, cfg.Policies.IdempotencyRetention)
	if err != nil {
		return application.Service{}, err
	}
	service := NewService(pool, idclock.SystemClock{}, idclock.NewMessageIDs("reservations"), Waits{Message: cfg.Wait, Command: cfg.Policies.IdempotencyWait})
	service.Idempotency = policy
	service.Instrumentation = usecase.New(rt, audit.NewLogSink(rt.LoggerProvider()), subject, classify, application.OperationFindReservation)
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
	service, err := NewReservationsService(pool, rt, cfg)
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

// NewReservationsConsumer is the consumer adapter with the attempt limit of the
// channel it consumes (ADR-039: the two must agree).
func NewReservationsConsumer(pool *pgxpool.Pool, cfg Config, ch channel.Channel, authenticated bool, tracer trace.Tracer, meters metric.MeterProvider, logs log.LoggerProvider) kernelapp.Consumer {
	return NewConsumer(pool, idclock.SystemClock{}, idclock.NewMessageIDs("reservations"), cfg.Wait, cfg.ConsumerTimeout, ch.Retry.MaxAttempts, OrdersBoundary(cfg, authenticated), ConsumerTelemetry{
		Tracer:         tracer,
		MeterProvider:  meters,
		LoggerProvider: logs,
		System:         semconv.MessagingSystemKafka.Value.AsString(),
		Channel:        kernelapp.Channel{Address: ch.Address, Group: ch.Group},
	})
}

// OrdersBoundary is the one place the consumer's trust is declared: the orders
// producer, and a transport called verified only when TLS checks the broker
// and this client authenticates to it (IDN-03, IDN-04).
func OrdersBoundary(cfg Config, authenticated bool) kernelapp.Boundary {
	transport := kernelapp.TransportDevelopmentOnly
	if authenticated {
		transport = kernelapp.TransportVerified
	}
	return kernelapp.Boundary{Transport: transport, Sources: []string{cfg.OrdersSource}}
}

func runConsumer(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
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
	if err := postgres.WaitForTables(ctx, pool, time.Second, rt.LoggerProvider(), append(postgres.Tables(postgres.Outbox, postgres.Inbox), "reservations")...); err != nil {
		return err
	}
	stopPurge, err := kernelapp.StartPurge(ctx, abort,
		kernelapp.PurgeConfig{Name: "message-inbox", Interval: cfg.Policies.PurgeInterval, Batch: cfg.Policies.PurgeBatch, Retention: cfg.Policies.InboxRetention},
		idclock.SystemClock{}, rt.LoggerProvider(), kernelapp.PurgeMessageInbox(pool, ConsumerName))
	if err != nil {
		return err
	}
	defer func() { _ = stopPurge() }()

	ch := OrdersChannel(cfg)
	catalog, err := channel.NewCatalog(ch)
	if err != nil {
		return err
	}
	kafkaConfig, err := kafka.NewConfig(ctx, rt, catalog, cfg.Brokers, cfg.KafkaInsecure, cfg.KafkaAuth)
	if err != nil {
		return err
	}
	logger := rt.LoggerFor(reflect.TypeFor[Config]().PkgPath())
	consumer := &kafka.Consumer{
		Config:  kafkaConfig,
		Channel: ch,
		Sink: Sink{
			Consumer:  NewReservationsConsumer(pool, cfg, ch, kafkaConfig.ClientAuthenticated(), rt.Tracer(), rt.MeterProvider(), rt.LoggerProvider()),
			EventType: channel.EventTypeOf(ch),
			Logger:    logger,
		},
		Backoff:            retry.Backoff{Base: 10 * time.Millisecond, Factor: 2, Cap: time.Second},
		ProcessingDeadline: processingDeadline,
		RebalanceTimeout:   rebalanceTimeout,
		QueuePerPartition:  queuePerPartition,
	}
	if err := consumer.Validate(); err != nil {
		return err
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "consumer joining",
		slog.String(string(semconv.MessagingSystemKey), semconv.MessagingSystemKafka.Value.AsString()),
		slog.String(string(semconv.MessagingDestinationNameKey), ch.Address),
		slog.String(string(semconv.MessagingConsumerGroupNameKey), ch.Group),
		slog.String(keyChannelName, ch.Name))
	// WHY: the relay returns nil on cancellation on its own, so guarding by
	// ctx.Err() here would also swallow a broker failure that happened to land
	// during the drain; only the cancellation itself is a clean exit.
	if err := consumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
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
	ch := ReservationsChannel(cfg)
	catalog, err := channel.NewCatalog(ch)
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

	drain, err := relay.NewOverPostgres(pool, publisher, "reservations", relay.Instrument(cfg.Relay, rt, catalog.AddressOf))
	if err != nil {
		return err
	}
	rt.LoggerFor(reflect.TypeFor[Config]().PkgPath()).LogAttrs(ctx, slog.LevelInfo, "relay draining",
		slog.String(string(semconv.MessagingSystemKey), semconv.MessagingSystemKafka.Value.AsString()),
		slog.String(string(semconv.MessagingDestinationNameKey), ch.Address),
		slog.String(keyChannelName, ch.Name))
	if err := drain.Run(ctx); err != nil {
		return err
	}
	return stopPurge()
}
