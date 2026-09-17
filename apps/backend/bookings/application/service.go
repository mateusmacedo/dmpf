package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/ports"
)

const (
	AggregateTypeBooking  = "bookings.Booking"
	AggregateTypeResource = "bookings.Resource"

	Destination = "bookings.events"

	OperationReserve        = "bookings.Reserve"
	OperationCancel         = "bookings.Cancel"
	OperationRegister       = "bookings.Register"
	OperationFindBooking    = "bookings.FindBooking"
	OperationFindByResource = "bookings.FindByResource"
)

const maxEventsPerCommand = 1

type Resources struct {
	Bookings  port.Repository[domain.BookingID, domain.BookingSnapshot]
	Resources port.Repository[domain.ResourceCode, domain.ResourceSnapshot]
	Outbox    port.Outbox
}

type Command interface{ isCommand() }

type Reserve struct {
	BookingID  domain.BookingID
	ResourceID domain.ResourceID
	Quantity   int
}

func (Reserve) isCommand() {}

type Cancel struct {
	BookingID domain.BookingID
}

func (Cancel) isCommand() {}

type Register struct {
	Code domain.ResourceCode
}

func (Register) isCommand() {}

type Service struct {
	UoW            port.UnitOfWork[Resources]
	Reader         port.Reader[domain.BookingID, domain.BookingSnapshot]
	ResourceReader ports.BookingsByResourceReader
	Clock          port.Clock
	IDs            port.IDGenerator
	Authorize      application.AuthorizeFunc[Command]
}

func (s Service) instrumentation() port.Instrumentation {
	return port.NoInstrumentation()
}

func authorizationResult(err error) port.Result {
	return port.Result{Outcome: port.OutcomeFailed, Err: err}
}

func enqueueAll(
	ctx context.Context,
	outbox port.Outbox,
	identity application.Identity,
	aggregateType string,
	aggregateID string,
	written port.Version,
	events []kernel.DomainEvent,
) error {
	if len(events) > len(identity.MessageIDs) {
		panic(fmt.Sprintf(
			"application: the decision produced %d events but only %d identifiers were resolved; raise maxEventsPerCommand",
			len(events), len(identity.MessageIDs)))
	}
	for i, event := range events {
		entry := port.OutboxEntry{
			MessageID:        identity.MessageIDs[i],
			OccurredAt:       identity.OccurredAt,
			Intent:           port.PublishIntent{Destination: Destination, PartitionKey: aggregateID},
			AggregateType:    aggregateType,
			AggregateID:      aggregateID,
			AggregateVersion: written,
			Event:            event,
			Context:          application.MessageContextFor(ctx, identity.MessageIDs[i]),
		}
		if err := outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}
