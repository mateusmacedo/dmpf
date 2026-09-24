package application

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

// Cancel cancels the pending reservation of Order, creating it when absent so a
// later OrderPlaced finds it canceled. A decided reservation refuses it.
func (s Service) Cancel(ctx context.Context, cmd Cancel) (application.Outcome[domain.CancelledResponse], error) {
	return write(ctx, s, OperationCancel, cmd, cmd.Order,
		func(r *domain.Reservation, at domain.Instant) (kernel.Accepted[domain.CancelledResponse], *kernel.Rejection) {
			return r.Cancel(domain.Cancel{At: at})
		})
}
