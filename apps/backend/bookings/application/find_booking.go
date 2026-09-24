package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
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

func (s Service) FindBookingByResource(ctx context.Context, resourceID domain.ResourceID) ([]domain.BookingSnapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindByResource)

	if err := s.Authorize(ctx, FindBookingByResource{Resource: resourceID}); err != nil {
		end(authorizationResult(err))
		return nil, err
	}

	snapshots, err := s.ResourceReader.LoadByResource(ctx, resourceID)
	var access ports.CrossTenantAccess
	switch {
	case errors.As(err, &access):
		// IDN-13: the caller gets the empty answer of a resource nobody holds;
		// the instrumentation still gets the access, which IDN-12 records.
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: access})
		return []domain.BookingSnapshot{}, nil
	case err != nil:
		failed := fmt.Errorf("application: find by resource %s: %w", resourceID, err)
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: failed})
		return nil, failed
	}

	end(ports.Result{Outcome: ports.OutcomeAccepted})
	return snapshots, nil
}
