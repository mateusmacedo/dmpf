package application

import (
	"context"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/ports"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	AggregateTypeBooking  = "bookings.Booking"
	AggregateTypeResource = "bookings.Resource"

	Destination = "bookings.events"

	CommandConsumer = "bookings.commands"

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
	Commands  port.Inbox
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
	Idempotency    usecase.IdempotencyPolicy

	Instrumentation port.Instrumentation
}

func idempotent[R any](
	ctx context.Context,
	s Service,
	res Resources,
	fingerprint *usecase.Fingerprint,
	operation string,
	now port.Instant,
	codec usecase.OutcomeCodec[R],
	run func() (usecase.Outcome[R], error),
) (usecase.Outcome[R], bool, error) {
	return usecase.RunIdempotent(ctx, usecase.IdempotentCommand[R]{
		Inbox:       res.Commands,
		Consumer:    CommandConsumer,
		Operation:   operation,
		Fingerprint: fingerprint,
		Now:         now,
		Policy:      s.Idempotency,
		Codec:       codec,
		Run:         run,
	})
}

// A nil hook is the inert realization, so every operation opens and closes
// unconditionally.
func (s Service) instrumentation() port.Instrumentation {
	if s.Instrumentation == nil {
		return port.NoInstrumentation()
	}
	return s.Instrumentation
}

func origin(aggregateType, aggregateID string) usecase.Origin {
	return usecase.Origin{Destination: Destination, AggregateType: aggregateType, AggregateID: aggregateID}
}
