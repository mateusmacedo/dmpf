package provider

import (
	"fmt"

	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/event/v1"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

const (
	reservationConfirmedType = "com.company.reservations.reservation-confirmed.v1"
	reservationCancelledType = "com.company.reservations.reservation-cancelled.v1"
)

// Mapper translates the reservations bounded context's domain events into
// their wire contracts.
type Mapper struct{}

func (Mapper) Map(event kernel.DomainEvent) (postgres.Mapped, error) {
	switch e := event.(type) {
	case domain.ReservationConfirmed:
		return postgres.Mapped{
			Message: &eventv1.ReservationConfirmed{
				OrderId:   string(e.Order),
				ItemCount: int32(e.Items),
			},
			Type: reservationConfirmedType,
		}, nil
	case domain.ReservationCancelled:
		return postgres.Mapped{
			Message: &eventv1.ReservationCancelled{OrderId: string(e.Order)},
			Type:    reservationCancelledType,
		}, nil
	default:
		return postgres.Mapped{}, fmt.Errorf("%w: %s", postgres.ErrUnmappedEvent, event.EventName())
	}
}
