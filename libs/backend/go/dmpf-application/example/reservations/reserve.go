package reservationsapp

import (
	"context"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/reservations"
)

// Reserve confirms Items for the reservation of Order, creating it when absent.
// A reservation already decided refuses it, and the refusal commits nothing.
func (s Service) Reserve(ctx context.Context, cmd Reserve) (dmpfapplication.Outcome[reservations.ReservedResponse], error) {
	return write(ctx, s, OperationReserve, cmd, cmd.Order,
		func(r *reservations.Reservation, at reservations.Instant) (dmpfdomain.Accepted[reservations.ReservedResponse], *dmpfdomain.Rejection) {
			return r.Reserve(reservations.Reserve{Items: cmd.Items, At: at})
		})
}
