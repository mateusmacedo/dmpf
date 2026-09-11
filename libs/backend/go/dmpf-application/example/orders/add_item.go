package ordersapp

import (
	"context"
	"errors"
	"fmt"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// AddItem walks the nine steps of FND-04 §3.2. Identity is resolved before the
// transaction opens, because a re-execution would mint new identity for the
// same fact (UOW-09).
func (s Service) AddItem(ctx context.Context, cmd AddItem) (dmpfapplication.Outcome[orders.ItemAccepted], error) {
	var zero dmpfapplication.Outcome[orders.ItemAccepted]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationAddItem)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := dmpfapplication.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		order, stored, err := s.loadOrCreate(ctx, res, cmd.Order)
		if err != nil {
			return err
		}

		accepted, rejection := order.AddItem(orders.AddItem{
			SKU:      cmd.SKU,
			Quantity: cmd.Quantity,
			At:       orders.Instant(identity.OccurredAt.Unix()),
		})
		if rejection != nil {
			// Returning nil commits a transaction with no effect, on purpose:
			// aborting would make a refusal indistinguishable from a technical
			// failure, which DEC-04 forbids (FND-04 §3.2).
			outcome = dmpfapplication.Rejected[orders.ItemAccepted](rejection)
			return nil
		}

		if err := res.Orders.Save(ctx, cmd.Order, order.Snapshot(), stored); err != nil {
			return err
		}
		if err := enqueueAll(ctx, res.Outbox, identity, cmd.Order, stored+1, accepted.Events()); err != nil {
			return err
		}

		outcome = dmpfapplication.Accepted(accepted.Response())
		return nil
	})
	if err != nil {
		end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: err})
		return zero, err
	}

	category := outcomeCategory(outcome)
	end(dmpfports.Result{Outcome: category})
	instrumentation.Audit(ctx, dmpfports.AuditEvent{
		Object:  string(cmd.Order),
		Action:  OperationAddItem,
		Outcome: category,
		At:      identity.OccurredAt,
	})
	return outcome, nil
}

// loadOrCreate is the "load or create" branch: ErrNotFound is not a failure
// here, it means the order does not exist yet and starts at version zero.
func (s Service) loadOrCreate(ctx context.Context, res Resources, id orders.OrderID) (*orders.Order, dmpfports.Version, error) {
	snapshot, stored, err := res.Orders.Load(ctx, id)
	switch {
	case errors.Is(err, dmpfports.ErrNotFound):
		return orders.NewOrder(id, s.ItemLimit), 0, nil
	case err != nil:
		return nil, 0, fmt.Errorf("ordersapp: add item to %s: %w", id, err)
	default:
		return orders.FromSnapshot(snapshot), stored, nil
	}
}
