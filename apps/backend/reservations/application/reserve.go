package application

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

// Reserve confirms Items for the reservation of Order, creating it when absent.
// A reservation already decided refuses it, and the refusal commits nothing.
func (s Service) Reserve(ctx context.Context, cmd Reserve) (application.Outcome[domain.ReservedResponse], error) {
	return write(ctx, s, OperationReserve, cmd, cmd.Order,
		func(r *domain.Reservation, at domain.Instant) (kernel.Accepted[domain.ReservedResponse], *kernel.Rejection) {
			return r.Reserve(domain.Reserve{Items: cmd.Items, At: at})
		})
}
