package app

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"

	httpedge "github.com/mateusmacedo/dmpf/apps/backend/bookings/app/http"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(ctx context.Context, cfg Config, out io.Writer) error {
	return boot.Boot(ctx, out, TelemetryOf(cfg), func(ctx context.Context, rt *otelboot.Runtime) error {
		return RunWith(ctx, cfg, rt)
	})
}

// RunWith is the delegate a caller that already owns the telemetry enters by,
// which is what the harnesses use.
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

func bindBookings(tx *postgres.Tx) application.Resources {
	return application.Resources{
		Bookings:  provider.NewBookingRepository(tx),
		Resources: provider.NewResourceRepository(tx),
		Outbox:    tx.Outbox(provider.Mapper{}),
	}
}

// NewBookingsService assembles the use cases over Postgres, which is the one
// place the concrete providers of this context are instantiated (ADR-015).
func NewBookingsService(pool *pgxpool.Pool) application.Service {
	return application.Service{
		UoW:            postgres.NewUnitOfWork(pool, bindBookings),
		Reader:         provider.NewBookingReader(pool),
		ResourceReader: provider.NewBookingsByResourceReader(pool),
		Clock:          idclock.SystemClock{},
		IDs:            idclock.NewMessageIDs("bookings"),
		Authorize:      usecase.AllowAllWithContext[application.Operation](),
	}
}

func NewMux(service application.Service, budget deadline.Budget) *http.ServeMux {
	return httpedge.Mux(service, budget)
}

func serveAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer())
	if err != nil {
		return err
	}
	defer pool.Close()
	if cfg.Migrate {
		if err := postgres.Migrate(ctx, pool); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: NewMux(NewBookingsService(pool), cfg.RouteBudget)}
	failed := make(chan error, 1)
	go func() { failed <- server.ListenAndServe() }()
	rt.Logger().InfoContext(ctx, "http listening", "addr", cfg.HTTPAddr)

	select {
	case <-ctx.Done():
		return drain(server, rt)
	case err := <-failed:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// drain stops accepting and lets the requests in flight finish, because a
// forced close cuts answers the caller is waiting for.
func drain(server *http.Server, rt *otelboot.Runtime) error {
	grace, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), observability.ShutdownGrace)
	defer cancel()
	if err := server.Shutdown(grace); err != nil {
		rt.Logger().WarnContext(grace, "http shutdown grace expired", "error", err.Error())
		return server.Close()
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

	catalog, err := NewCatalog(cfg)
	if err != nil {
		return err
	}
	if err := postgres.AssertOwnOutbox(ctx, pool, slices.Collect(maps.Keys(catalog))); err != nil {
		return err
	}
	publisher, err := kafka.NewPublisher(
		kafka.NewConfig(ctx, rt, catalog, cfg.Brokers, cfg.Service, cfg.KafkaInsecure), nil)
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
