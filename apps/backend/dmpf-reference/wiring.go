package dmpfreference

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference/api"
	dmpfapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-app"
	reservationsconsumer "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-app/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-app/relay"
	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	ordersapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/orders"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/usecase"
	dmpfkafka "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-kafka"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
	orderspg "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/channel"
)

const (
	// admissionKeys bounds the (route, tenant) buckets (MET-07): three routes
	// and one tenant need far less, and the ceiling is what the controller demands.
	admissionKeys = 64

	// The server bounds every phase of a request on its own: the header, the
	// body, the write and the idle keep-alive. The deadline budget of the
	// routes governs the outbound side (RST-03); these govern the inbound one.
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 16
	shutdownGrace     = 10 * time.Second

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
	defer func() { _ = rt.Shutdown(context.WithoutCancel(ctx)) }()

	return RunWith(ctx, cfg, rt, out)
}

// RunWith runs the role over a runtime the caller booted: otelboot starts once
// per process, so a test that hosts the three roles in one process shares one.
func RunWith(ctx context.Context, cfg Config, rt *otelboot.Runtime, out io.Writer) error {
	switch cfg.Role {
	case RoleAPI:
		return serveAPI(ctx, cfg, rt, out)
	case RoleRelay:
		return runRelay(ctx, cfg, rt, out)
	case RoleConsumer:
		return runConsumer(ctx, cfg, rt, out)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownRole, cfg.Role)
	}
}

// NewPool opens the connection pool and proves it reaches the database before
// any role starts: a process that cannot reach Postgres has nothing to do.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// bindOrders binds the resource set of the orders use case to the open
// transaction: repository and outbox on the same pgx.Tx (UOW-01, UOW-08).
func bindOrders(tx *dmpfpostgres.Tx) ordersapp.Resources {
	return ordersapp.Resources{
		Orders: orderspg.NewRepository(tx),
		Outbox: tx.Outbox(orderspg.Mapper{}),
	}
}

// NewOrdersService assembles the reference application service over Postgres,
// with the instrumentation of FND-08 and the audit trail written to auditOut.
func NewOrdersService(pool *pgxpool.Pool, rt *otelboot.Runtime, cfg Config, auditOut io.Writer) ordersapp.Service {
	return ordersapp.Service{
		UoW:             dmpfpostgres.NewUnitOfWork(pool, bindOrders),
		Reader:          orderspg.NewReader(pool),
		Clock:           SystemClock{},
		IDs:             RandomMessageIDs{},
		Authorize:       dmpfapplication.AllowAll[ordersapp.Command](),
		ItemLimit:       cfg.ItemLimit,
		Instrumentation: usecase.New(rt, NewAuditSink(auditOut, cfg), subject, classify, ordersapp.OperationFindOrder),
	}
}

// NewAdmission builds the controller of RES-16 for the three routes, one limit
// each, over the single tenant this edge knows.
func NewAdmission(limit admission.Limit) (*admission.Controller, error) {
	tenants, err := metrics.DeclareTenants(api.Tenant)
	if err != nil {
		return nil, err
	}
	return admission.New(admission.Config{
		Limits:  api.Limits(limit),
		Tenants: tenants,
		MaxKeys: admissionKeys,
		Clock:   obsclock.System(),
	})
}

func serveAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime, out io.Writer) error {
	pool, err := NewPool(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.Migrate {
		if err := dmpfpostgres.Migrate(ctx, pool); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	ctrl, err := NewAdmission(cfg.Admission)
	if err != nil {
		return err
	}
	opts := api.Options{Budget: cfg.Budget, CORSOrigins: cfg.CORSOrigins}
	if cfg.OpenAPIPath != "" {
		document, err := os.ReadFile(cfg.OpenAPIPath)
		if err != nil {
			return fmt.Errorf("openapi: %w", err)
		}
		opts.OpenAPI = document
	}
	handler, err := api.NewHandler(NewOrdersService(pool, rt, cfg, out), ctrl, rt.Tracer(), rt.Instruments(), opts)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
	failed := make(chan error, 1)
	go func() { failed <- server.ListenAndServe() }()
	rt.Logger().InfoContext(ctx, "api listening", "addr", cfg.HTTPAddr)

	select {
	case <-ctx.Done():
		grace, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
		defer cancel()
		return server.Shutdown(grace)
	case err := <-failed:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// NewRelay assembles the drain of FND-04 §5.4 over Postgres: claim by lease,
// publish, mark. It runs in its own process, apart from the request path (BLK-02).
func NewRelay(pool *pgxpool.Pool, publisher relay.Publisher, cfg relay.Config) (relay.Relay, error) {
	clock := SystemClock{}
	return relay.New(dmpfpostgres.NewOutboxStore(pool, clock), publisher, RandomClaimIDs{}, clock, cfg)
}

func runRelay(ctx context.Context, cfg Config, rt *otelboot.Runtime, _ io.Writer) error {
	pool, err := NewPool(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	catalog, err := NewCatalog(cfg)
	if err != nil {
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
	rt.Logger().InfoContext(ctx, "relay draining", "channels", len(catalog), "orders", cfg.Topic, "reservations", cfg.ReservationsTopic)
	return drain.Run(ctx)
}

// NewReservationsConsumer is the consumer adapter of the example, with the
// attempt limit of the channel it consumes (ADR-039: the two must agree).
func NewReservationsConsumer(pool *pgxpool.Pool, cfg Config, ch channel.Channel) dmpfapp.Consumer {
	return reservationsconsumer.NewConsumer(pool, SystemClock{}, RandomMessageIDs{}, cfg.Wait, ch.Retry.MaxAttempts)
}

func runConsumer(ctx context.Context, cfg Config, rt *otelboot.Runtime, _ io.Writer) error {
	pool, err := NewPool(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	catalog, err := NewCatalog(cfg)
	if err != nil {
		return err
	}
	ch := OrdersChannel(cfg)
	consumer := &dmpfkafka.Consumer{
		Config:  NewKafkaConfig(ctx, cfg, rt, catalog),
		Channel: ch,
		Sink: Sink{
			Consumer:  NewReservationsConsumer(pool, cfg, ch),
			EventType: EventTypeOf(ch),
			Logger:    rt.Logger(),
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
	if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
