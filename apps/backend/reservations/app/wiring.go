package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/provider"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/serve"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	obsusecase "github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

const (

	// processingDeadline must stay below rebalanceTimeout (KFK-19): a
	// revocation waits for the record in flight, never for the whole queue.
	processingDeadline = 5 * time.Second
	rebalanceTimeout   = 30 * time.Second
	queuePerPartition  = 32
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
		return serve.ServeAPI(ctx, rt, apiRole(cfg), api(cfg, rt))
	case RoleRelay:
		return serve.RunRelay(ctx, rt, relayRole(cfg), serve.Relay{Component: "reservations", Channel: ReservationsChannel(cfg), Scope: reflect.TypeFor[Config]().PkgPath()})
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
	service.Instrumentation = obsusecase.New(rt, audit.NewLogSink(rt.LoggerProvider()), ports.SubjectOf, usecase.CategoryOf, application.OperationFindReservation)
	return service, nil
}

func api(cfg Config, rt *otelboot.Runtime) serve.API {
	return serve.API{
		Desc: &rpc.ServiceDesc, Commands: rpc.Commands(), Schema: provider.Schema, Grants: provider.Grants, CommandConsumer: application.CommandConsumer,
		Register: func(server *grpc.Server, pool *pgxpool.Pool) error {
			service, err := NewReservationsService(pool, rt, cfg)
			if err != nil {
				return err
			}
			server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: service})
			return nil
		},
	}
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
	if err := postgres.WaitForGrants(ctx, pool, time.Second, rt.LoggerProvider(), append(postgres.KernelGrants(postgres.Outbox, postgres.Inbox), provider.Grants...)...); err != nil {
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
		slog.String(logging.KeyChannelName, ch.Name))
	// WHY: the relay returns nil on cancellation on its own, so guarding by
	// ctx.Err() here would also swallow a broker failure that happened to land
	// during the drain; only the cancellation itself is a clean exit.
	if err := consumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return stopPurge()
}
