package bookingsapplication

import (
	"context"
	"fmt"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

func (s Service) FindBooking(ctx context.Context, id bookingsdomain.BookingID) (bookingsdomain.BookingSnapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindBooking)

	snapshot, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		failed := fmt.Errorf("bookingsapplication: find booking %s: %w", id, err)
		end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: failed})
		return bookingsdomain.BookingSnapshot{}, failed
	}

	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})
	return snapshot, nil
}

func (s Service) FindBookingByResource(ctx context.Context, resourceID bookingsdomain.ResourceID) ([]bookingsdomain.BookingSnapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindByResource)

	snapshots, err := s.ResourceReader.LoadByResource(ctx, resourceID)
	if err != nil {
		failed := fmt.Errorf("bookingsapplication: find by resource %s: %w", resourceID, err)
		end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: failed})
		return nil, failed
	}

	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})
	return snapshots, nil
}
