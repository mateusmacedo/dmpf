package application

import (
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

type command[R any] = usecase.Command[Resources, Operation, R]

func (s Service) executor() usecase.Executor[Resources, Operation] {
	return usecase.Executor[Resources, Operation]{
		UoW:             s.UoW,
		Inbox:           func(res Resources) port.Inbox { return res.Commands },
		Consumer:        CommandConsumer,
		Clock:           s.Clock,
		IDs:             s.IDs,
		MaxEvents:       maxEventsPerCommand,
		Policy:          s.Idempotency,
		Authorize:       s.Authorize,
		Instrumentation: s.Instrumentation,
	}
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

var (
	loadExistingBooking = usecase.Existing[domain.BookingID](domain.FromBookingSnapshot)
	loadAbsentBooking   = usecase.Absent[domain.BookingSnapshot](domain.NewBooking)
	loadResource        = usecase.OrNew(domain.NewResource, domain.FromResourceSnapshot)
)
