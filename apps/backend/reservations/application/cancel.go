package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// Cancel cancels the pending reservation of Order, creating it when absent so a
// later OrderPlaced finds it canceled. A decided reservation refuses it.
func (s Service) Cancel(ctx context.Context, cmd Cancel) (usecase.Outcome[domain.CancelledResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.CancelledResponse]{
		Operation:   OperationCancel,
		Object:      string(cmd.Order),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationCancel).String(string(cmd.Order)),
		Codec:       cancelledCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.CancelledResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Reservations, res.Outbox, origin(cmd.Order), cmd.Order, identity, loadReservation,
				func(r *domain.Reservation) (kernel.Accepted[domain.CancelledResponse], *kernel.Rejection) {
					return r.Cancel(domain.Cancel{At: domain.Instant(identity.OccurredAt)})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: cancel %s: %w", cmd.Order, err)
			}
			return outcome, nil
		},
	})
}
