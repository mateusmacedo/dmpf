package serve

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"google.golang.org/grpc"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

var (
	ErrServiceNotRegistered = errors.New("serve: Register did not register the service specified by Desc")
	ErrRelayUnnamed         = errors.New("serve: the relay needs a component and a scope")
)

type APIRole struct {
	DSN           string
	Server        kernelgrpc.APIEnv
	Migrate       bool
	Admission     admission.Limit
	MetricTenants []string
	Policies      kernelapp.Policies
}

// API takes the registration as a function, not as a service value, so a
// generated context with no service yet still serves (ADR-061).
type API struct {
	Desc            *grpc.ServiceDesc
	Commands        []string
	Schema          string
	CommandConsumer string
	Register        func(*grpc.Server, *pgxpool.Pool) error
}

func (api API) methods() []string {
	names := make([]string, 0, len(api.Desc.Methods))
	for _, method := range api.Desc.Methods {
		names = append(names, method.MethodName)
	}
	return names
}

type RelayRole struct {
	DSN           string
	Brokers       []string
	KafkaInsecure bool
	KafkaAuth     kafka.ClientAuth
	Relay         relay.Config
	Policies      kernelapp.Policies
}

type Relay struct {
	Component string
	Channel   channel.Channel
	Scope     string
}

func ServeAPI(ctx context.Context, rt *otelboot.Runtime, cfg APIRole, api API) error {
	ctx, abort := context.WithCancelCause(ctx)
	defer abort(nil)
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer(), postgres.WithMeterProvider(rt.MeterProvider()))
	if err != nil {
		return err
	}
	defer pool.Close()
	stopPurge := func() error { return nil }
	defer func() { _ = stopPurge() }()

	ctrl, err := admission.NewController(kernelgrpc.MethodLimits(api.Desc.ServiceName, api.methods(), cfg.Admission), cfg.MetricTenants, admission.DefaultMaxKeys)
	if err != nil {
		return err
	}
	serverConfig, err := kernelgrpc.APIServerConfig(kernelgrpc.APIServer{
		CertFile:       cfg.Server.GRPCCertFile,
		KeyFile:        cfg.Server.GRPCKeyFile,
		ClientCAFile:   cfg.Server.GRPCClientCAFile,
		TrustedClients: cfg.Server.GRPCTrustedClients,
		Insecure:       cfg.Server.GRPCInsecure,
		Services:       kernelgrpc.HealthServices(api.Desc.ServiceName),
		Interceptors:   kernelgrpc.ServerInterceptors(api.Desc.ServiceName, ctrl, rt.Instruments(), rt.LoggerProvider(), kernelgrpc.WithCommands(api.Commands...)),
		LoggerProvider: rt.LoggerProvider(),
	})
	if err != nil {
		return err
	}
	server, healthServer, err := kernelgrpc.NewServer(serverConfig)
	if err != nil {
		return err
	}
	if err := api.Register(server, pool); err != nil {
		return err
	}
	if _, ok := server.GetServiceInfo()[api.Desc.ServiceName]; !ok {
		return fmt.Errorf("%w: %s", ErrServiceNotRegistered, api.Desc.ServiceName)
	}

	listen := func() (net.Listener, error) { return net.Listen("tcp", cfg.Server.GRPCAddr) }
	ready := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		if cfg.Migrate {
			if err := postgres.Migrate(ctx, pool, []postgres.Capability{postgres.Outbox, postgres.Inbox}, api.Schema); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
		}
		stop, err := kernelapp.StartPurge(ctx, abort,
			kernelapp.PurgeConfig{Name: "command-inbox", Interval: cfg.Policies.PurgeInterval, Batch: cfg.Policies.PurgeBatch},
			idclock.SystemClock{}, rt.LoggerProvider(), kernelapp.PurgeCommandInbox(pool, api.CommandConsumer))
		if err != nil {
			return err
		}
		stopPurge = stop
		return nil
	}
	if err := kernelgrpc.Serve(ctx, listen, server, healthServer, kernelgrpc.HealthServices(api.Desc.ServiceName), ready, rt.LoggerProvider()); err != nil {
		return err
	}
	return stopPurge()
}

func RunRelay(ctx context.Context, rt *otelboot.Runtime, cfg RelayRole, r Relay) error {
	if r.Component == "" || r.Scope == "" {
		return ErrRelayUnnamed
	}
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

	catalog, err := channel.NewCatalog(r.Channel)
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

	drain, err := relay.NewOverPostgres(pool, publisher, r.Component, relay.Instrument(cfg.Relay, rt, catalog.AddressOf))
	if err != nil {
		return err
	}
	rt.LoggerFor(r.Scope).LogAttrs(ctx, slog.LevelInfo, "relay draining",
		slog.String(string(semconv.MessagingSystemKey), semconv.MessagingSystemKafka.Value.AsString()),
		slog.String(string(semconv.MessagingDestinationNameKey), r.Channel.Address),
		slog.String(logging.KeyChannelName, r.Channel.Name))
	if err := drain.Run(ctx); err != nil {
		return err
	}
	return stopPurge()
}
