package reservationsapp

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/reservations"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// FindReservation reads through Reader, never through UoW: a query neither
// opens a transaction nor writes the outbox (UOW-11).
func (s Service) FindReservation(ctx context.Context, id reservations.OrderID) (reservations.Snapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindReservation)

	snapshot, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		failed := fmt.Errorf("reservationsapp: find reservation %s: %w", id, err)
		end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: failed})
		return reservations.Snapshot{}, failed
	}

	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})
	return snapshot, nil
}
