// comment-discipline-ok-file: arquivo de declarações da composition root; cada godoc é contrato de API pública com referência normativa (FND-04 §6.3, MAP-07, ERR-11), dentro do limite de 3 linhas.

package reservationsconsumer

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	reservationsapp "github.com/mateusmacedo/dmpf/libs/backend/go/application/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	reservationspg "github.com/mateusmacedo/dmpf/libs/backend/go/postgres/example/reservations"
)

// ConsumerName is the logical, stable identity of INB-01: shared by every
// replica, never a hostname or an instance id.
const ConsumerName = "reservations"

// Bind is the composition root's half of ADR-034: it turns one open
// transaction into the resource set reservationsapp declares, so inbox,
// business state and outbox share the same pgx.Tx (INB-07).
func Bind(wait time.Duration) func(tx *postgres.Tx) reservationsapp.Resources {
	return func(tx *postgres.Tx) reservationsapp.Resources {
		return reservationsapp.Resources{
			Inbox:        tx.Inbox(ConsumerName, wait),
			Reservations: reservationspg.NewRepository(tx),
			Outbox:       tx.Outbox(reservationspg.Mapper{}),
		}
	}
}

// NewService assembles the application service over Postgres. wait is the
// ceiling of INB-17 the caller declares; its value is FND-08's.
func NewService(pool *pgxpool.Pool, clock ports.Clock, ids ports.IDGenerator, wait time.Duration) reservationsapp.Service {
	return reservationsapp.Service{
		UoW:       postgres.NewUnitOfWork(pool, Bind(wait)),
		Clock:     clock,
		IDs:       ids,
		Authorize: application.AllowAll[reservationsapp.Command](),
		Consumer:  ConsumerName,
	}
}

// Handler unpacks OrderPlaced and hands it to Consume. A payload that is not
// this consumer's contract is terminal (Validation, ERR-11): nothing would be
// written under R1×D4, so no unit of work is opened for it.
func Handler(service reservationsapp.Service) app.Handler {
	return func(ctx context.Context, receipt ports.Receipt, env envelope.Envelope) (application.Disposition, error) {
		var placed eventv1.OrderPlaced
		if err := envelope.Unpack(env, &placed); err != nil {
			return application.R1D4, application.NewFailure(application.Validation, false, err)
		}
		return service.Consume(ctx, reservationsapp.ConsumeOrderPlaced{
			MessageID:   receipt.MessageID,
			MessageType: receipt.MessageType,
			PayloadHash: receipt.PayloadHash,
			ReceivedAt:  receipt.ReceivedAt,
			Order:       reservations.OrderID(placed.GetOrderId()),
			Items:       int(placed.GetItemCount()),
		})
	}
}

// NewConsumer is the whole consumer: adapter, service and Postgres quarantine.
// maxAttempts <= 0 disables the attempt limit (GAR-08 fixes that one exists;
// the value is FND-08's).
func NewConsumer(pool *pgxpool.Pool, clock ports.Clock, ids ports.IDGenerator, wait time.Duration, maxAttempts int) app.Consumer {
	return app.Consumer{
		Name:        ConsumerName,
		MaxAttempts: maxAttempts,
		Handle:      Handler(NewService(pool, clock, ids, wait)),
		Containment: postgres.NewQuarantine(pool),
		Clock:       clock,
	}
}
