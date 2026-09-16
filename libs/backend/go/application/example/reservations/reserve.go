package reservationsapp

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
)

// Reserve confirms Items for the reservation of Order, creating it when absent.
// A reservation already decided refuses it, and the refusal commits nothing.
func (s Service) Reserve(ctx context.Context, cmd Reserve) (application.Outcome[reservations.ReservedResponse], error) {
	return write(ctx, s, OperationReserve, cmd, cmd.Order,
		func(r *reservations.Reservation, at reservations.Instant) (domain.Accepted[reservations.ReservedResponse], *domain.Rejection) {
			return r.Reserve(reservations.Reserve{Items: cmd.Items, At: at})
		})
}
