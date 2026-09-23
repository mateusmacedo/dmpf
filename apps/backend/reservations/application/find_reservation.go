package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

// FindReservation reads through Reader, never through UoW: a query neither
// opens a transaction nor writes the outbox (UOW-11). It still walks step 1
// first, because authenticating at the route is not permission to read.
func (s Service) FindReservation(ctx context.Context, id domain.OrderID) (domain.Snapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindReservation)

	if err := s.Authorize(ctx, FindReservation{Order: id}); err != nil {
		end(authorizationResult(err))
		return domain.Snapshot{}, err
	}

	snapshot, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		failed := fmt.Errorf("application: find reservation %s: %w", id, err)
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: failed})
		return domain.Snapshot{}, failed
	}

	end(ports.Result{Outcome: ports.OutcomeAccepted})
	return snapshot, nil
}
