// comment-discipline-ok-file: arquivo de declarações da composition root; cada godoc é contrato de API pública com referência normativa (FND-04 §6.3, MAP-07, ERR-11), dentro do limite de 3 linhas.

package app

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	eventv1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/provider"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// ConsumerName is the logical, stable identity of INB-01: shared by every
// replica, never a hostname or an instance id.
const ConsumerName = "reservations"

// Waits are the ceilings of INB-17 for a message and of IDM-07 for a command;
// a role that runs no command leaves Command zero.
type Waits struct {
	Message time.Duration
	Command time.Duration
}

// Bind is the composition root's half of ADR-034: it turns one open
// transaction into the resource set application declares, so inbox,
// business state and outbox share the same pgx.Tx (INB-07).
func Bind(waits Waits) func(tx *postgres.Tx) application.Resources {
	return func(tx *postgres.Tx) application.Resources {
		return application.Resources{
			Inbox:        tx.Inbox(ConsumerName, waits.Message),
			Reservations: provider.NewReservationRepository(tx),
			Outbox:       tx.Outbox(provider.Mapper{}),
			Commands:     tx.CommandInbox(application.CommandConsumer, waits.Command),
		}
	}
}

// NewService assembles the application service over Postgres, with the waits
// the caller declares; their values are FND-08's.
func NewService(pool *pgxpool.Pool, clock ports.Clock, ids ports.IDGenerator, waits Waits) application.Service {
	return application.Service{
		UoW:       postgres.NewUnitOfWork(pool, Bind(waits)),
		Reader:    provider.NewReservationReader(postgres.NewReadPool(pool)),
		Clock:     clock,
		IDs:       ids,
		Authorize: Authorization(),
		Consumer:  ConsumerName,
	}
}

// Handler unpacks OrderPlaced and hands it to Consume. A payload that is not
// this consumer's contract is terminal (Validation, ERR-11): nothing would be
// written under R1×D4, so no unit of work is opened for it.
func Handler(service application.Service) kernelapp.Handler {
	return func(ctx context.Context, receipt ports.Receipt, env envelope.Envelope) (usecase.Disposition, error) {
		var placed eventv1.OrderPlaced
		if err := envelope.Unpack(env, &placed); err != nil {
			return usecase.R1D4, usecase.NewFailure(usecase.Validation, false, err)
		}
		return service.ConsumeOrderPlaced(ctx, application.ConsumeOrderPlaced{
			MessageID:   receipt.MessageID,
			MessageType: receipt.MessageType,
			PayloadHash: receipt.PayloadHash,
			ReceivedAt:  receipt.ReceivedAt,
			Order:       domain.OrderID(placed.GetOrderId()),
			Items:       int(placed.GetItemCount()),
		})
	}
}

// consumerLocale answers a mandatory field of CTX-01 that a consumption has no
// source for: there is no caller stating a preference on this side.
const consumerLocale = "en"

type ConsumerTelemetry struct {
	Tracer         trace.Tracer
	MeterProvider  metric.MeterProvider
	LoggerProvider log.LoggerProvider
	System         string
	Channel        kernelapp.Channel
}

// NewConsumer is the whole consumer: adapter, service and Postgres quarantine.
// maxAttempts <= 0 disables the attempt limit (GAR-08 fixes that one exists;
// the value is FND-08's).
func NewConsumer(pool *pgxpool.Pool, clock ports.Clock, ids ports.IDGenerator, wait, timeout time.Duration, maxAttempts int, boundary kernelapp.Boundary, telemetry ConsumerTelemetry) kernelapp.Consumer {
	return kernelapp.Consumer{
		Name:           ConsumerName,
		MaxAttempts:    maxAttempts,
		Handle:         Handler(NewService(pool, clock, ids, Waits{Message: wait})),
		Containment:    postgres.NewQuarantine(pool, postgres.WithQuarantineLoggerProvider(telemetry.LoggerProvider)),
		Clock:          clock,
		Timeout:        timeout,
		Boundary:       boundary,
		Locale:         consumerLocale,
		Tracer:         telemetry.Tracer,
		MeterProvider:  telemetry.MeterProvider,
		LoggerProvider: telemetry.LoggerProvider,
		System:         telemetry.System,
		Channel:        telemetry.Channel,
	}
}
