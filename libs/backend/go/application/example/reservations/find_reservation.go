package reservationsapp

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// FindReservation reads through Reader, never through UoW: a query neither
// opens a transaction nor writes the outbox (UOW-11).
func (s Service) FindReservation(ctx context.Context, id reservations.OrderID) (reservations.Snapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindReservation)

	snapshot, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		failed := fmt.Errorf("reservationsapp: find reservation %s: %w", id, err)
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: failed})
		return reservations.Snapshot{}, failed
	}

	end(ports.Result{Outcome: ports.OutcomeAccepted})
	return snapshot, nil
}
