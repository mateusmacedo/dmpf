package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
)

func (s Service) FindBooking(ctx context.Context, id domain.BookingID) (domain.BookingSnapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindBooking)

	snapshot, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		failed := fmt.Errorf("application: find booking %s: %w", id, err)
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: failed})
		return domain.BookingSnapshot{}, failed
	}

	end(ports.Result{Outcome: ports.OutcomeAccepted})
	return snapshot, nil
}

func (s Service) FindBookingByResource(ctx context.Context, resourceID domain.ResourceID) ([]domain.BookingSnapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindByResource)

	snapshots, err := s.ResourceReader.LoadByResource(ctx, resourceID)
	if err != nil {
		failed := fmt.Errorf("application: find by resource %s: %w", resourceID, err)
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: failed})
		return nil, failed
	}

	end(ports.Result{Outcome: ports.OutcomeAccepted})
	return snapshots, nil
}
