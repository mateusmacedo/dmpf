package bookingspostgres

import (
	"fmt"
	"time"

	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/bookings/event/v1"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
	"google.golang.org/protobuf/types/known/timestamppb"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

const bookingReservedType = "com.company.bookings.booking-reserved.v1"

type Mapper struct{}

func (Mapper) Map(event dmpfdomain.DomainEvent) (dmpfpostgres.Mapped, error) {
	switch e := event.(type) {
	case bookingsdomain.BookingReserved:
		return dmpfpostgres.Mapped{
			Message: &eventv1.BookingReserved{
				BookingId:  string(e.BookingID),
				ResourceId: string(e.ResourceID),
				Quantity:   int32(e.Quantity),
				ReservedAt: timestamppb.New(time.Unix(int64(e.At), 0)),
			},
			Type: bookingReservedType,
		}, nil
	default:
		return dmpfpostgres.Mapped{}, fmt.Errorf("%w: %s", dmpfpostgres.ErrUnmappedEvent, event.EventName())
	}
}
