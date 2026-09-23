package app

import (
	"context"
	"fmt"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"io"
	"maps"
	"net"
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	obsusecase "github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

const ()

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
	default:
		return fmt.Errorf("%w: %q", ErrUnknownRole, cfg.Role)
	}
}

func bindOrders(tx *postgres.Tx) application.Resources {
	return application.Resources{
		Orders: provider.NewRepository(tx),
		Outbox: tx.Outbox(provider.Mapper{}),
	}
}

// NewOrdersService assembles the orders use cases over Postgres, with the
// instrumentation of FND-08 and the audit trail written to auditOut.
func NewOrdersService(pool *pgxpool.Pool, rt *otelboot.Runtime, cfg Config, auditOut io.Writer) application.Service {
	return application.Service{
		UoW:             postgres.NewUnitOfWork(pool, bindOrders),
		Reader:          provider.NewReader(postgres.NewReadPool(pool)),
		Clock:           idclock.SystemClock{},
		IDs:             idclock.NewMessageIDs("orders"),
		Authorize:       Authorization(),
		ItemLimit:       cfg.ItemLimit,
		Instrumentation: obsusecase.New(rt, audit.NewEnvelopeSink(auditOut, audit.Identity{Service: cfg.Service, Version: cfg.Version, Instance: cfg.Instance}), subject, classify, application.OperationFindOrder),
	}
}

func serveAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime, out io.Writer) error {
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer())
	if err != nil {
		return err
	}
	defer pool.Close()

	ctrl, err := admission.NewController(rpc.Limits(cfg.Admission), cfg.MetricTenants, admission.DefaultMaxKeys)
	if err != nil {
		return err
	}
	serverConfig, err := kernel.APIServerConfig(kernel.APIServer{
		CertFile:     cfg.GRPCCertFile,
		KeyFile:      cfg.GRPCKeyFile,
		Insecure:     cfg.GRPCInsecure,
		Services:     kernel.HealthServices(rpc.ServiceName),
		Interceptors: rpc.Interceptors(rt.Tracer(), ctrl, rt.Instruments(), rt.Logger()),
		Logger:       rt.Logger(),
	})
	if err != nil {
		return err
	}
	server, healthServer, err := kernel.NewServer(serverConfig)
	if err != nil {
		return err
	}
	server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: NewOrdersService(pool, rt, cfg, out)})

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
	publisher, err := kafka.NewPublisher(kafka.NewConfig(ctx, rt, catalog, cfg.Brokers, cfg.Service, cfg.KafkaInsecure), nil)
	if err != nil {
		return err
	}
	defer publisher.Close()

	drain, err := relay.NewOverPostgres(pool, publisher, "orders", cfg.Relay)
	if err != nil {
		return err
	}
	rt.Logger().InfoContext(ctx, "relay draining", "channel", application.Destination, "topic", cfg.OrdersTopic)
	return drain.Run(ctx)
}
