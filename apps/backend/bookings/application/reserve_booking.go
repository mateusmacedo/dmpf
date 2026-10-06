package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (s Service) ReserveBooking(ctx context.Context, cmd ReserveBooking) (usecase.Outcome[domain.ReservedResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ReservedResponse]{
		Operation: OperationReserveBooking,
		Object:    string(cmd.BookingID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationReserveBooking).
			String(string(cmd.BookingID)).String(string(cmd.ResourceID)).Int(int64(cmd.Quantity)),
		Codec: reservedCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ReservedResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Bookings, res.Outbox, origin(AggregateTypeBooking, string(cmd.BookingID)), cmd.BookingID, identity,
				loadAbsentBooking,
				func(b *domain.Booking) (kernel.Accepted[domain.ReservedResponse], *kernel.Rejection) {
					return b.Reserve(domain.ReserveBooking{ResourceID: cmd.ResourceID, Quantity: cmd.Quantity, At: domain.Instant(identity.OccurredAt)})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: reserve %s: %w", cmd.BookingID, err)
			}
			return outcome, nil
		},
	})
}
