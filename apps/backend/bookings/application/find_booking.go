package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func (s Service) FindBooking(ctx context.Context, id domain.BookingID) (domain.BookingSnapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindBooking)

	if err := s.Authorize(ctx, FindBooking{Booking: id}); err != nil {
		end(authorizationResult(err))
		return domain.BookingSnapshot{}, err
	}

	snapshot, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		failed := fmt.Errorf("application: find booking %s: %w", id, err)
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: failed})
		return domain.BookingSnapshot{}, failed
	}

	end(ports.Result{Outcome: ports.OutcomeAccepted})
	return snapshot, nil
}
