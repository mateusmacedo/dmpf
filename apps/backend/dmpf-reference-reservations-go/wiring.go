package dmpfreferencereservations

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference-reservations-go/rpc"
	dmpfapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-app"
	reservationsconsumer "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-app/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-app/relay"
	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	reservationsapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/reservations"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/usecase"
	dmpfgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-grpc"
	dmpfkafka "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-kafka"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
	reservationspg "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/channel"
)

const (
	admissionKeys = 64
	shutdownGrace = 10 * time.Second

	// processingDeadline must stay below rebalanceTimeout (KFK-19): a
	// revocation waits for the record in flight, never for the whole queue.
	processingDeadline = 5 * time.Second
	rebalanceTimeout   = 30 * time.Second
	queuePerPartition  = 32
)

// Run boots the telemetry of the process and runs cfg.Role until ctx is
// cancelled or the role fails. Every concrete provider is built below this
// call and nowhere else in the workspace (ADR-015).
func Run(ctx context.Context, cfg Config, out io.Writer) error {
	rt, err := NewTelemetry(ctx, cfg, out)
	if err != nil {
		return err
	}
	defer shutdownTelemetry(ctx, rt)

	return RunWith(ctx, cfg, rt, out)
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

// NewPool builds the pool with the query tracer of the process. It does not
// reach the database: readiness is the ping that follows.
func NewPool(ctx context.Context, dsn string, tracer trace.Tracer) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.ConnConfig.Tracer = dbTracer{tracer: tracer}
	return pgxpool.NewWithConfig(ctx, config)
}

// NewReservationsService assembles the synchronous reservations use cases over
// Postgres, with the resource set the consumer also binds (INB-07).
func NewReservationsService(pool *pgxpool.Pool, rt *otelboot.Runtime, cfg Config, auditOut io.Writer) reservationsapp.Service {
	return reservationsapp.Service{
		UoW:             dmpfpostgres.NewUnitOfWork(pool, reservationsconsumer.Bind(cfg.Wait)),
		Reader:          reservationspg.NewReader(pool),
		Clock:           SystemClock{},
		IDs:             RandomMessageIDs{},
		Authorize:       dmpfapplication.AllowAll[reservationsapp.Command](),
		Consumer:        reservationsconsumer.ConsumerName,
		Instrumentation: usecase.New(rt, NewAuditSink(auditOut, cfg), subject, classify, reservationsapp.OperationFindReservation),
	}
}

// NewAdmission builds the controller of RES-16, one limit per method.
func NewAdmission(limit admission.Limit) (*admission.Controller, error) {
	tenants, err := metrics.DeclareTenants(rpc.Tenant)
	if err != nil {
		return nil, err
	}
	return admission.New(admission.Config{
		Limits:  rpc.Limits(limit),
		Tenants: tenants,
		MaxKeys: admissionKeys,
		Clock:   obsclock.System(),
	})
}

func serverTLS(cfg Config) (*tls.Config, error) {
	if cfg.GRPCCertFile == "" {
		return nil, nil
	}
	certificate, err := tls.LoadX509KeyPair(cfg.GRPCCertFile, cfg.GRPCKeyFile)
	if err != nil {
		return nil, fmt.Errorf("grpc tls: %w", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}, nil
}

func apiServerConfig(cfg Config, rt *otelboot.Runtime, ctrl *admission.Controller) (dmpfgrpc.ServerConfig, error) {
	tlsConfig, err := serverTLS(cfg)
	if err != nil {
		return dmpfgrpc.ServerConfig{}, err
	}
	return dmpfgrpc.ServerConfig{
		TLS:                        tlsConfig,
		InsecureForDevelopmentOnly: tlsConfig == nil && cfg.GRPCInsecure,
		Services:                   healthServices(),
		UnaryInterceptors:          rpc.Interceptors(rt.Tracer(), ctrl, rt.Instruments(), rt.Logger()),
		Logger:                     rt.Logger(),
	}, nil
}

func serveAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime, out io.Writer) error {
	pool, err := NewPool(ctx, cfg.DSN, rt.Tracer())
	if err != nil {
		return err
	}
	defer pool.Close()

	ctrl, err := NewAdmission(cfg.Admission)
	if err != nil {
		return err
	}
	serverConfig, err := apiServerConfig(cfg, rt, ctrl)
	if err != nil {
		return err
	}
	server, healthServer, err := dmpfgrpc.NewServer(serverConfig)
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
			if err := dmpfpostgres.Migrate(ctx, pool); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
		}
		return nil
	}
	return serveGRPC(ctx, listen, server, healthServer, healthServices(), ready, rt.Logger())
}

// healthServices is what the probe may ask for. The empty name is the overall
// status, which health.NewServer starts as SERVING: a pod whose schema is not
// applied yet would pass the probe without it.
func healthServices() []string { return []string{"", rpc.ServiceName} }

// serveGRPC runs ready before the first connection is accepted, so a probe that
// cannot speak the health protocol — a TLS listener refuses the kubelet's gRPC
// probe, which is why hmg falls back to tcpSocket — still only succeeds on a
// process that answered its dependencies.
func serveGRPC(ctx context.Context, listen func() (net.Listener, error), server *grpc.Server, healthServer *health.Server, services []string, ready func(context.Context) error, logger *slog.Logger) error {
	if err := ready(ctx); err != nil {
		return err
	}
	listener, err := listen()
	if err != nil {
		return err
	}

	failed := make(chan error, 1)
	go func() { failed <- server.Serve(listener) }()
	logger.InfoContext(ctx, "grpc listening", "addr", listener.Addr().String())
	for _, service := range services {
		healthServer.SetServingStatus(service, healthpb.HealthCheckResponse_SERVING)
	}

	select {
	case <-ctx.Done():
		drainGRPC(ctx, server, healthServer, logger)
		return nil
	case err := <-failed:
		drainGRPC(ctx, server, healthServer, logger)
		return err
	}
}

// drainGRPC stops accepting, lets the calls in flight finish and reports when
// the window expires, because a forced Stop cuts answers the caller is waiting
// for and a silent exit is indistinguishable from a clean one.
func drainGRPC(ctx context.Context, server *grpc.Server, healthServer *health.Server, logger *slog.Logger) {
	healthServer.Shutdown()
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(shutdownGrace):
		logger.WarnContext(ctx, "grpc shutdown grace expired", "grace", shutdownGrace.String())
		server.Stop()
	}
}

// NewRelay assembles the drain of FND-04 §5.4 over the reservations outbox. It
// runs in its own process, apart from the request path (BLK-02).
func NewRelay(pool *pgxpool.Pool, publisher relay.Publisher, cfg relay.Config) (relay.Relay, error) {
	clock := SystemClock{}
	return relay.New(dmpfpostgres.NewOutboxStore(pool, clock), publisher, RandomClaimIDs{}, clock, cfg)
}

func runRelay(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	pool, err := NewPool(ctx, cfg.DSN, rt.Tracer())
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
	if err := assertOwnOutbox(ctx, pool, catalog); err != nil {
		return err
	}
	publisher, err := dmpfkafka.NewPublisher(NewKafkaConfig(ctx, cfg, rt, catalog), nil)
	if err != nil {
		return err
	}
	defer publisher.Close()

	drain, err := NewRelay(pool, publisher, cfg.Relay)
	if err != nil {
		return err
	}
	rt.Logger().InfoContext(ctx, "relay draining", "channel", ch.Name, "topic", ch.Address)
	return drain.Run(ctx)
}

// NewReservationsConsumer is the consumer adapter with the attempt limit of the
// channel it consumes (ADR-039: the two must agree).
func NewReservationsConsumer(pool *pgxpool.Pool, cfg Config, ch channel.Channel) dmpfapp.Consumer {
	return reservationsconsumer.NewConsumer(pool, SystemClock{}, RandomMessageIDs{}, cfg.Wait, ch.Retry.MaxAttempts)
}

func runConsumer(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	pool, err := NewPool(ctx, cfg.DSN, rt.Tracer())
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
	consumer := &dmpfkafka.Consumer{
		Config:  NewKafkaConfig(ctx, cfg, rt, catalog),
		Channel: ch,
		Sink: Sink{
			Consumer:  NewReservationsConsumer(pool, cfg, ch),
			EventType: EventTypeOf(ch),
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

// WHY: context.WithoutCancel drops the cancellation and adds no deadline, and
// otelboot.Runtime.Shutdown hands the context straight to both providers, so an
// unreachable collector blocks the exit until the orchestrator sends SIGKILL.
func shutdownTelemetry(ctx context.Context, rt *otelboot.Runtime) {
	grace, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
	defer cancel()
	if err := rt.Shutdown(grace); err != nil {
		rt.Logger().WarnContext(grace, "telemetry shutdown", "error", err.Error())
	}
}

// WHY: relay.Store.Claim does not filter by destination, so a DSN shared
// between contexts makes this relay claim the other's records, fail to resolve
// the channel and bury them as failed after the attempts run out.
//
// The query stops at the first foreign row and looks only at what the relay
// would claim, which is what dmpf_outbox_claim_idx already covers: a DISTINCT
// over the whole table would scan every retained record on boot.
func assertOwnOutbox(ctx context.Context, pool *pgxpool.Pool, catalog channel.Catalog) error {
	mine := make([]string, 0, len(catalog))
	for destination := range catalog {
		mine = append(mine, destination)
	}
	const query = `SELECT destination FROM dmpf_outbox
		WHERE status IN ('pending', 'publishing') AND destination <> ALL($1)
		LIMIT 1`

	var foreign string
	switch err := pool.QueryRow(ctx, query, mine).Scan(&foreign); {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("outbox destinations: %w", err)
	default:
		return fmt.Errorf("outbox holds %q, which this context does not publish: the database is shared with another context", foreign)
	}
}
