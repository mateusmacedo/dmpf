package reservationspg

import (
	"fmt"

	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	reservationConfirmedType = "com.company.reservations.reservation-confirmed.v1"
	reservationCancelledType = "com.company.reservations.reservation-cancelled.v1"
)

// Mapper translates the reservations bounded context's domain events into
// their wire contracts.
type Mapper struct{}

func (Mapper) Map(event domain.DomainEvent) (postgres.Mapped, error) {
	switch e := event.(type) {
	case reservations.ReservationConfirmed:
		return postgres.Mapped{
			Message: &reservationsv1.ReservationConfirmed{
				OrderId:   string(e.Order),
				ItemCount: int32(e.Items),
			},
			Type: reservationConfirmedType,
		}, nil
	case reservations.ReservationCancelled:
		return postgres.Mapped{
			Message: &reservationsv1.ReservationCancelled{OrderId: string(e.Order)},
			Type:    reservationCancelledType,
		}, nil
	default:
		return postgres.Mapped{}, fmt.Errorf("%w: %s", postgres.ErrUnmappedEvent, event.EventName())
	}
}
