package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"io"
	"maps"
	"net"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
)

const (

	// processingDeadline must stay below rebalanceTimeout (KFK-19): a
	// revocation waits for the record in flight, never for the whole queue.
	processingDeadline = 5 * time.Second
	rebalanceTimeout   = 30 * time.Second
	queuePerPartition  = 32
)

func Run(ctx context.Context, cfg Config, out io.Writer) error {
	return boot.Boot(ctx, out, TelemetryOf(cfg), func(ctx context.Context, rt *otelboot.Runtime) error {
		return RunWith(ctx, cfg, rt, out)
	})
}

// RunWith runs the role over a runtime the caller booted.
func RunWith(ctx context.Context, cfg Config, rt *otelboot.Runtime, out io.Writer) error {
	switch cfg.Role {
	case RoleAPI:
		return serveAPI(ctx, cfg, rt, out)
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
func NewReservationsService(pool *pgxpool.Pool, rt *otelboot.Runtime, cfg Config, auditOut io.Writer) application.Service {
	service := NewService(pool, idclock.SystemClock{}, idclock.NewMessageIDs("reservations"), cfg.Wait)
	service.Instrumentation = usecase.New(rt, audit.NewEnvelopeSink(auditOut, audit.Identity{Service: cfg.Service, Version: cfg.Version, Instance: cfg.Instance, Tenant: rpc.Tenant}), subject, classify, application.OperationFindReservation)
	return service
}

func serveAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime, out io.Writer) error {
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer())
	if err != nil {
		return err
	}
	defer pool.Close()

	ctrl, err := admission.NewController(rpc.Limits(cfg.Admission), rpc.Tenant, admission.DefaultMaxKeys)
	if err != nil {
		return err
	}
	serverConfig, err := provider.APIServerConfig(provider.APIServer{
		CertFile:     cfg.GRPCCertFile,
		KeyFile:      cfg.GRPCKeyFile,
		Insecure:     cfg.GRPCInsecure,
		Services:     provider.HealthServices(rpc.ServiceName),
		Interceptors: rpc.Interceptors(rt.Tracer(), ctrl, rt.Instruments(), rt.Logger()),
		Logger:       rt.Logger(),
	})
	if err != nil {
		return err
	}
	server, healthServer, err := provider.NewServer(serverConfig)
	if err != nil {
		return err
	}
	server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: NewReservationsService(pool, rt, cfg, out)})

	listen := func() (net.Listener, error) { return net.Listen("tcp", cfg.GRPCAddr) }
	ready := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		if cfg.Migrate {
			if err := postgres.Migrate(ctx, pool); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
		}
		return nil
	}
	return provider.Serve(ctx, listen, server, healthServer, provider.HealthServices(rpc.ServiceName), ready, rt.Logger())
}

// NewReservationsConsumer is the consumer adapter with the attempt limit of the
// channel it consumes (ADR-039: the two must agree).
func NewReservationsConsumer(pool *pgxpool.Pool, cfg Config, ch channel.Channel) app.Consumer {
	return NewConsumer(pool, idclock.SystemClock{}, idclock.NewMessageIDs("reservations"), cfg.Wait, ch.Retry.MaxAttempts)
}

func runConsumer(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer())
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}

	ch := OrdersChannel(cfg)
	catalog, err := NewCatalog(ch)
	if err != nil {
		return err
	}
	consumer := &kafka.Consumer{
		Config:  kafka.NewConfig(ctx, rt, catalog, cfg.Brokers, cfg.Service, cfg.KafkaInsecure),
		Channel: ch,
		Sink: Sink{
			Consumer:  NewReservationsConsumer(pool, cfg, ch),
			EventType: channel.EventTypeOf(ch),
			Logger:    rt.Logger(),
			Tracer:    rt.Tracer(),
		},
		Backoff:            retry.Backoff{Base: 10 * time.Millisecond, Factor: 2, Cap: time.Second},
		ProcessingDeadline: processingDeadline,
		RebalanceTimeout:   rebalanceTimeout,
		QueuePerPartition:  queuePerPartition,
	}
	if err := consumer.Validate(); err != nil {
		return err
	}
	rt.Logger().InfoContext(ctx, "consumer joining", "channel", ch.Name, "topic", ch.Address, "group", ch.Group)
	// WHY: the relay returns nil on cancellation on its own, so guarding by
	// ctx.Err() here would also swallow a broker failure that happened to land
	// during the drain; only the cancellation itself is a clean exit.
	if err := consumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
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

	ch := ReservationsChannel(cfg)
	catalog, err := NewCatalog(ch)
	if err != nil {
		return err
	}
	if err := postgres.AssertOwnOutbox(ctx, pool, slices.Collect(maps.Keys(catalog))); err != nil {
		return err
	}
	publisher, err := kafka.NewPublisher(kafka.NewConfig(ctx, rt, catalog, cfg.Brokers, cfg.Service, cfg.KafkaInsecure), nil)
	if err != nil {
		return err
	}
	defer publisher.Close()

	drain, err := relay.NewOverPostgres(pool, publisher, "reservations", cfg.Relay)
	if err != nil {
		return err
	}
	rt.Logger().InfoContext(ctx, "relay draining", "channel", ch.Name, "topic", ch.Address)
	return drain.Run(ctx)
}
