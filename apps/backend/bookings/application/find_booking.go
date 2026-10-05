package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

func (s Service) FindBooking(ctx context.Context, id domain.BookingID) (domain.BookingSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindBooking, FindBooking{Booking: id},
		func(ctx context.Context) (domain.BookingSnapshot, error) {
			snapshot, _, err := s.Reader.Load(ctx, id)
			if err != nil {
				return domain.BookingSnapshot{}, fmt.Errorf("application: find booking %s: %w", id, err)
			}
			return snapshot, nil
		})
}
