package app

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/provider"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/serve"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	obsusecase "github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
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
		return serve.RunRelay(ctx, rt, relayRole(cfg), serve.Relay{Component: "orders", Channel: OrdersChannel(cfg), Scope: reflect.TypeFor[Config]().PkgPath()})
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
	service.Instrumentation = obsusecase.New(rt, audit.NewLogSink(rt.LoggerProvider()), ports.SubjectOf, usecase.CategoryOf, application.OperationFindOrder)
	return service, nil
}

func api(cfg Config, rt *otelboot.Runtime) serve.API {
	return serve.API{
		Desc: &rpc.ServiceDesc, Commands: rpc.Commands(), Schema: provider.Schema, CommandConsumer: application.CommandConsumer,
		Register: func(server *grpc.Server, pool *pgxpool.Pool) error {
			service, err := NewOrdersService(pool, rt, cfg)
			if err != nil {
				return err
			}
			server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: service})
			return nil
		},
	}
}
