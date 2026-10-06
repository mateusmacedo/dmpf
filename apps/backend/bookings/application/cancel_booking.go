package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (s Service) CancelBooking(ctx context.Context, cmd CancelBooking) (usecase.Outcome[domain.CancelledResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.CancelledResponse]{
		Operation:   OperationCancelBooking,
		Object:      string(cmd.BookingID),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationCancelBooking).String(string(cmd.BookingID)),
		Codec:       cancelledCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.CancelledResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Bookings, res.Outbox, origin(AggregateTypeBooking, string(cmd.BookingID)), cmd.BookingID, identity,
				loadExistingBooking,
				func(b *domain.Booking) (kernel.Accepted[domain.CancelledResponse], *kernel.Rejection) {
					return b.Cancel(domain.CancelBooking{At: domain.Instant(identity.OccurredAt)})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: cancel %s: %w", cmd.BookingID, err)
			}
			return outcome, nil
		},
	})
}
