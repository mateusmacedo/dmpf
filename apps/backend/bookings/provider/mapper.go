package provider

import (
	"fmt"
	"time"

	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/bookings/event/v1"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
)

const (
	bookingReservedType    = "com.company.bookings.booking-reserved.v1"
	bookingCancelledType   = "com.company.bookings.booking-cancelled.v1"
	resourceRegisteredType = "com.company.bookings.resource-registered.v1"
)

type Mapper struct{}

func (Mapper) Map(event kernel.DomainEvent) (postgres.Mapped, error) {
	switch e := event.(type) {
	case domain.BookingReserved:
		return postgres.Mapped{
			Message: &eventv1.BookingReserved{
				BookingId:  string(e.BookingID),
				ResourceId: string(e.ResourceID),
				Quantity:   int32(e.Quantity),
				ReservedAt: timestamppb.New(time.Unix(0, int64(e.At))),
			},
			Type: bookingReservedType,
		}, nil
	case domain.BookingCancelledEvent:
		return postgres.Mapped{
			Message: &eventv1.BookingCancelled{
				BookingId:   string(e.BookingID),
				CancelledAt: timestamppb.New(time.Unix(0, int64(e.At))),
			},
			Type: bookingCancelledType,
		}, nil
	case domain.ResourceRegistered:
		return postgres.Mapped{
			Message: &eventv1.ResourceRegistered{
				ResourceId:   string(e.Code),
				RegisteredAt: timestamppb.New(time.Unix(0, int64(e.At))),
			},
			Type: resourceRegisteredType,
		}, nil
	default:
		return postgres.Mapped{}, fmt.Errorf("%w: %s", postgres.ErrUnmappedEvent, event.EventName())
	}
}
