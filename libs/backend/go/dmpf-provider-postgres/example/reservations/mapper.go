package reservationspg

import (
	"fmt"

	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/reservations/event/v1"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/reservations"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

const (
	reservationConfirmedType = "com.company.reservations.reservation-confirmed.v1"
	reservationCancelledType = "com.company.reservations.reservation-cancelled.v1"
)

// Mapper translates the reservations bounded context's domain events into
// their wire contracts.
type Mapper struct{}

func (Mapper) Map(event dmpfdomain.DomainEvent) (dmpfpostgres.Mapped, error) {
	switch e := event.(type) {
	case reservations.ReservationConfirmed:
		return dmpfpostgres.Mapped{
			Message: &reservationsv1.ReservationConfirmed{
				OrderId:   string(e.Order),
				ItemCount: int32(e.Items),
			},
			Type: reservationConfirmedType,
		}, nil
	case reservations.ReservationCancelled:
		return dmpfpostgres.Mapped{
			Message: &reservationsv1.ReservationCancelled{OrderId: string(e.Order)},
			Type:    reservationCancelledType,
		}, nil
	default:
		return dmpfpostgres.Mapped{}, fmt.Errorf("%w: %s", dmpfpostgres.ErrUnmappedEvent, event.EventName())
	}
}
