package bookingsapplication

import (
	"context"
	"fmt"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
	bookingsports "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/ports"
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
	Bookings  dmpfports.Repository[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot]
	Resources dmpfports.Repository[bookingsdomain.ResourceCode, bookingsdomain.ResourceSnapshot]
	Outbox    dmpfports.Outbox
}

type Command interface{ isCommand() }

type Reserve struct {
	BookingID  bookingsdomain.BookingID
	ResourceID bookingsdomain.ResourceID
	Quantity   int
}

func (Reserve) isCommand() {}

type Cancel struct {
	BookingID bookingsdomain.BookingID
}

func (Cancel) isCommand() {}

type Register struct {
	Code bookingsdomain.ResourceCode
}

func (Register) isCommand() {}

type Service struct {
	UoW            dmpfports.UnitOfWork[Resources]
	Reader         dmpfports.Reader[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot]
	ResourceReader bookingsports.BookingsByResourceReader
	Clock          dmpfports.Clock
	IDs            dmpfports.IDGenerator
	Authorize      dmpfapplication.AuthorizeFunc[Command]
}

func (s Service) instrumentation() dmpfports.Instrumentation {
	return dmpfports.NoInstrumentation()
}

func authorizationResult(err error) dmpfports.Result {
	return dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: err}
}

func enqueueAll(
	ctx context.Context,
	outbox dmpfports.Outbox,
	identity dmpfapplication.Identity,
	aggregateType string,
	aggregateID string,
	written dmpfports.Version,
	events []dmpfdomain.DomainEvent,
) error {
	if len(events) > len(identity.MessageIDs) {
		panic(fmt.Sprintf(
			"bookingsapplication: the decision produced %d events but only %d identifiers were resolved; raise maxEventsPerCommand",
			len(events), len(identity.MessageIDs)))
	}
	for i, event := range events {
		entry := dmpfports.OutboxEntry{
			MessageID:        identity.MessageIDs[i],
			OccurredAt:       identity.OccurredAt,
			Intent:           dmpfports.PublishIntent{Destination: Destination, PartitionKey: aggregateID},
			AggregateType:    aggregateType,
			AggregateID:      aggregateID,
			AggregateVersion: written,
			Event:            event,
			Context:          dmpfapplication.MessageContextFor(ctx, identity.MessageIDs[i]),
		}
		if err := outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}
