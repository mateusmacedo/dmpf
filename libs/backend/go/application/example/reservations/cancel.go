package reservationsapp

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
)

// Cancel cancels the pending reservation of Order, creating it when absent so a
// later OrderPlaced finds it canceled. A decided reservation refuses it.
func (s Service) Cancel(ctx context.Context, cmd Cancel) (application.Outcome[reservations.CancelledResponse], error) {
	return write(ctx, s, OperationCancel, cmd, cmd.Order,
		func(r *reservations.Reservation, at reservations.Instant) (domain.Accepted[reservations.CancelledResponse], *domain.Rejection) {
			return r.Cancel(reservations.Cancel{At: at})
		})
}
