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

const bookingReservedType = "com.company.bookings.booking-reserved.v1"

type Mapper struct{}

func (Mapper) Map(event kernel.DomainEvent) (postgres.Mapped, error) {
	switch e := event.(type) {
	case domain.BookingReserved:
		return postgres.Mapped{
			Message: &eventv1.BookingReserved{
				BookingId:  string(e.BookingID),
				ResourceId: string(e.ResourceID),
				Quantity:   int32(e.Quantity),
				ReservedAt: timestamppb.New(time.Unix(int64(e.At), 0)),
			},
			Type: bookingReservedType,
		}, nil
	default:
		return postgres.Mapped{}, fmt.Errorf("%w: %s", postgres.ErrUnmappedEvent, event.EventName())
	}
}
