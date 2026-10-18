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
	ErrNoRuntime            = errors.New("serve: no runtime")
	ErrAPIIncomplete        = errors.New("serve: the API needs Desc and Register")
	ErrNoMethods            = errors.New("serve: Desc declares no method yet; declare the first one in the contract before serving the api role")
	ErrStreamsNotServed     = errors.New("serve: Desc declares streams, which the unary chain of ServeAPI does not cover")
	ErrUnknownCommand       = errors.New("serve: a command is not a method of Desc")
	ErrServiceNotRegistered = errors.New("serve: Register did not register the service specified by Desc")
	ErrUnexpectedService    = errors.New("serve: Register registered a service other than the one specified by Desc")
	ErrRelayUnnamed         = errors.New("serve: the relay needs a component and a scope")
)

// Ping and Migrate run before the port listens, so without a deadline a stuck
// connection or a migration lock held by another replica leaves the process
// neither serving nor failing.
const (
	pingTimeout    = 10 * time.Second
	migrateTimeout = 2 * time.Minute
	grantsTimeout  = 15 * time.Second
	uniquesTimeout = 15 * time.Second
)

type APIRole struct {
	DSN           string
	MigrateDSN    string
	Server        kernelgrpc.APIEnv
	Migrate       bool
	Admission     admission.Limit
	MetricTenants []string
	Policies      kernelapp.Policies
}

// API takes the registration as a function because the service is built over
// the pool that ServeAPI opens.
type API struct {
	Desc            *grpc.ServiceDesc
	Commands        []string
	Schema          string
	Grants          []postgres.Grant
	Uniques         []postgres.UniqueSet
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

func (api API) validate() error {
	switch {
	case api.Desc == nil || api.Register == nil:
		return ErrAPIIncomplete
	case len(api.Desc.Methods) == 0:
		return ErrNoMethods
	case len(api.Desc.Streams) > 0:
		return ErrStreamsNotServed
	}
	methods := api.methods()
	for _, command := range api.Commands {
		if !slices.Contains(methods, command) {
			return fmt.Errorf("%w: %s", ErrUnknownCommand, command)
		}
	}
	return nil
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
	if rt == nil {
		return ErrNoRuntime
	}
	if err := api.validate(); err != nil {
		return err
	}
	return stopped(ctx, serveAPI(ctx, rt, cfg, api))
}

func serveAPI(ctx context.Context, rt *otelboot.Runtime, cfg APIRole, api API) error {
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
	if err := register(server, pool, api); err != nil {
		return err
	}

	listen := func() (net.Listener, error) { return net.Listen("tcp", cfg.Server.GRPCAddr) }
	ready := func(ctx context.Context) error {
		if err := ping(ctx, pool); err != nil {
			return err
		}
		if err := prepare(ctx, rt, pool, cfg, api); err != nil {
			return err
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

// The ownMethods filter of the chain lets any other service through without
// admission or deadline, so Register may add only the service of Desc.
func register(server *grpc.Server, pool *pgxpool.Pool, api API) error {
	before := server.GetServiceInfo()
	if err := api.Register(server, pool); err != nil {
		return err
	}
	after := server.GetServiceInfo()
	if _, ok := after[api.Desc.ServiceName]; !ok {
		return fmt.Errorf("%w: %s", ErrServiceNotRegistered, api.Desc.ServiceName)
	}
	for name := range after {
		if _, existed := before[name]; !existed && name != api.Desc.ServiceName {
			return fmt.Errorf("%w: %s", ErrUnexpectedService, name)
		}
	}
	return nil
}

func prepare(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool, cfg APIRole, api API) error {
	if err := migrateOrWait(ctx, rt, pool, cfg, api); err != nil {
		return err
	}
	return verifyUniques(ctx, rt, pool, cfg.Migrate, api.Uniques)
}

// A replica without MIGRATE may start before the migrator adds a constraint, so
// it waits for the catalog to converge; after its own migration it checks once.
func verifyUniques(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool, migrated bool, sets []postgres.UniqueSet) error {
	if len(sets) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, uniquesTimeout)
	defer cancel()
	if migrated {
		return postgres.VerifyUniques(ctx, pool, sets...)
	}
	return postgres.WaitForUniques(ctx, pool, time.Second, rt.LoggerProvider(), sets...)
}

func migrateOrWait(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool, cfg APIRole, api API) error {
	capabilities := []postgres.Capability{postgres.Outbox, postgres.Inbox}
	grants := append(postgres.KernelGrants(capabilities...), api.Grants...)
	if !cfg.Migrate {
		return postgres.WaitForGrants(ctx, pool, time.Second, rt.LoggerProvider(), grants...)
	}
	ctx, cancel := context.WithTimeout(ctx, migrateTimeout)
	defer cancel()
	if cfg.MigrateDSN != "" {
		runtimeSession, err := postgres.SessionOf(ctx, pool)
		if err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		if err := migrateAs(ctx, cfg.MigrateDSN, runtimeSession, capabilities, api.Schema, grants); err != nil {
			return err
		}
		return awaitGrants(ctx, pool, rt.LoggerProvider(), grantsTimeout, grants)
	}
	if err := postgres.Migrate(ctx, pool, capabilities, api.Schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func ping(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	return nil
}

// A stop requested before the port listens is a clean exit, not a failure; an
// abort of the purge cancels only the derived context, so it stays an error.
func stopped(parent context.Context, err error) error {
	if parent.Err() != nil && errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func RunRelay(ctx context.Context, rt *otelboot.Runtime, cfg RelayRole, r Relay) error {
	switch {
	case rt == nil:
		return ErrNoRuntime
	case r.Component == "" || r.Scope == "":
		return ErrRelayUnnamed
	}
	catalog, err := channel.NewCatalog(r.Channel)
	if err != nil {
		return err
	}
	return stopped(ctx, runRelay(ctx, rt, cfg, r, catalog))
}

func runRelay(ctx context.Context, rt *otelboot.Runtime, cfg RelayRole, r Relay, catalog channel.Catalog) error {
	ctx, abort := context.WithCancelCause(ctx)
	defer abort(nil)
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer(), postgres.WithMeterProvider(rt.MeterProvider()))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := ping(ctx, pool); err != nil {
		return err
	}
	if err := postgres.WaitForGrants(ctx, pool, time.Second, rt.LoggerProvider(), postgres.KernelGrants(postgres.Outbox)...); err != nil {
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
