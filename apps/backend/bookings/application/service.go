package application

import (
	"context"
	"errors"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/ports"
)

const (
	AggregateTypeBooking  = "bookings.Booking"
	AggregateTypeResource = "bookings.Resource"

	Destination = "bookings.events"

	OperationReserveBooking        = "bookings.ReserveBooking"
	OperationCancelBooking         = "bookings.CancelBooking"
	OperationRegisterResource      = "bookings.RegisterResource"
	OperationFindBooking           = "bookings.FindBooking"
	OperationFindBookingByResource = "bookings.FindBookingByResource"
)

const maxEventsPerCommand = 1

type Resources struct {
	Bookings  port.Repository[domain.BookingID, domain.BookingSnapshot]
	Resources port.Repository[domain.ResourceCode, domain.ResourceSnapshot]
	Outbox    port.Outbox
}

type Operation interface{ isOperation() }

type ReserveBooking struct {
	BookingID  domain.BookingID
	ResourceID domain.ResourceID
	Quantity   int
}

func (ReserveBooking) isOperation() {}

type CancelBooking struct {
	BookingID domain.BookingID
}

func (CancelBooking) isOperation() {}

type RegisterResource struct {
	Code domain.ResourceCode
}

func (RegisterResource) isOperation() {}

// FindBooking and FindBookingByResource ask for state without changing it. A
// query is an entry point like any other: authenticating at the route does not
// stand for permission to read what the operation returns.
type FindBooking struct {
	Booking domain.BookingID
}

func (FindBooking) isOperation() {}

type FindBookingByResource struct {
	Resource domain.ResourceID
}

func (FindBookingByResource) isOperation() {}

type Service struct {
	UoW            port.UnitOfWork[Resources]
	Reader         port.Reader[domain.BookingID, domain.BookingSnapshot]
	ResourceReader ports.BookingsByResourceReader
	Clock          port.Clock
	IDs            port.IDGenerator
	Authorize      usecase.Authorize[Operation]

	Instrumentation port.Instrumentation
}

// A nil hook is the inert realization, so every operation opens and closes
// unconditionally.
func (s Service) instrumentation() port.Instrumentation {
	if s.Instrumentation == nil {
		return port.NoInstrumentation()
	}
	return s.Instrumentation
}

// authorizationResult categorises a step 1 error. Only a declared denial is
// Denied; anything else is technical failure, because inferring a refusal from
// an unrelated error would report a false negative of access.
func authorizationResult(err error) port.Result {
	if errors.Is(err, port.ErrDenied) {
		return port.Result{Outcome: port.OutcomeDenied}
	}
	return port.Result{Outcome: port.OutcomeFailed, Err: err}
}

// outcomeCategory reads the terminal category off the outcome, which is the
// only place that knows which branch of the UPR was taken.
func outcomeCategory[R any](outcome usecase.Outcome[R]) port.OutcomeCategory {
	if _, refused := outcome.Rejection(); refused {
		return port.OutcomeRejected
	}
	return port.OutcomeAccepted
}

func enqueueAll(
	ctx context.Context,
	outbox port.Outbox,
	identity usecase.Identity,
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
			Context:          usecase.MessageContextFor(ctx, identity.MessageIDs[i]),
		}
		if err := outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}
