package reservationsapp

import (
	"context"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/reservations"
)

// Cancel cancels the pending reservation of Order, creating it when absent so a
// later OrderPlaced finds it canceled. A decided reservation refuses it.
func (s Service) Cancel(ctx context.Context, cmd Cancel) (dmpfapplication.Outcome[reservations.CancelledResponse], error) {
	return write(ctx, s, OperationCancel, cmd, cmd.Order,
		func(r *reservations.Reservation, at reservations.Instant) (dmpfdomain.Accepted[reservations.CancelledResponse], *dmpfdomain.Rejection) {
			return r.Cancel(reservations.Cancel{At: at})
		})
}
