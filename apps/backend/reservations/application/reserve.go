package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// Reserve confirms Items for the reservation of Order, creating it when absent.
// A reservation already decided refuses it, and the refusal commits nothing.
func (s Service) Reserve(ctx context.Context, cmd Reserve) (usecase.Outcome[domain.ReservedResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ReservedResponse]{
		Operation:   OperationReserve,
		Object:      string(cmd.Order),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationReserve).String(string(cmd.Order)).Int(int64(cmd.Items)),
		Codec:       reservedCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ReservedResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Reservations, res.Outbox, origin(cmd.Order), cmd.Order, identity, loadReservation,
				func(r *domain.Reservation) (kernel.Accepted[domain.ReservedResponse], *kernel.Rejection) {
					return r.Reserve(domain.Reserve{Items: cmd.Items, At: domain.Instant(identity.OccurredAt)})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: reserve %s: %w", cmd.Order, err)
			}
			return outcome, nil
		},
	})
}
