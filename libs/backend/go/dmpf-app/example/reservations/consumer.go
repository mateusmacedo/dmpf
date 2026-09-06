// comment-discipline-ok-file: arquivo de declarações da composition root; cada godoc é contrato de API pública com referência normativa (FND-04 §6.3, MAP-07, ERR-11), dentro do limite de 3 linhas.

package reservationsconsumer

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-app"
	dmpfapplication "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application"
	reservationsapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/reservations"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/reservations"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
	reservationspg "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres/example/reservations"
)

// ConsumerName is the logical, stable identity of INB-01: shared by every
// replica, never a hostname or an instance id.
const ConsumerName = "reservations"

// Bind is the composition root's half of ADR-034: it turns one open
// transaction into the resource set reservationsapp declares, so inbox,
// business state and outbox share the same pgx.Tx (INB-07).
func Bind(wait time.Duration) func(tx *dmpfpostgres.Tx) reservationsapp.Resources {
	return func(tx *dmpfpostgres.Tx) reservationsapp.Resources {
		return reservationsapp.Resources{
			Inbox:        tx.Inbox(ConsumerName, wait),
			Reservations: reservationspg.NewRepository(tx),
			Outbox:       tx.Outbox(reservationspg.Mapper{}),
		}
	}
}

// NewService assembles the application service over Postgres. wait is the
// ceiling of INB-17 the caller declares; its value is FND-08's.
func NewService(pool *pgxpool.Pool, clock dmpfports.Clock, ids dmpfports.IDGenerator, wait time.Duration) reservationsapp.Service {
	return reservationsapp.Service{
		UoW:       dmpfpostgres.NewUnitOfWork(pool, Bind(wait)),
		Clock:     clock,
		IDs:       ids,
		Authorize: dmpfapplication.AllowAll[reservationsapp.Command](),
		Consumer:  ConsumerName,
	}
}

// Handler unpacks OrderPlaced and hands it to Consume. A payload that is not
// this consumer's contract is terminal (Validation, ERR-11): nothing would be
// written under R1×D4, so no unit of work is opened for it.
func Handler(service reservationsapp.Service) dmpfapp.Handler {
	return func(ctx context.Context, receipt dmpfports.Receipt, env envelope.Envelope) (dmpfapplication.Disposition, error) {
		var placed eventv1.OrderPlaced
		if err := envelope.Unpack(env, &placed); err != nil {
			return dmpfapplication.R1D4, dmpfapplication.NewFailure(dmpfapplication.Validation, false, err)
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
func NewConsumer(pool *pgxpool.Pool, clock dmpfports.Clock, ids dmpfports.IDGenerator, wait time.Duration, maxAttempts int) dmpfapp.Consumer {
	return dmpfapp.Consumer{
		Name:        ConsumerName,
		MaxAttempts: maxAttempts,
		Handle:      Handler(NewService(pool, clock, ids, wait)),
		Containment: dmpfpostgres.NewQuarantine(pool),
		Clock:       clock,
	}
}
