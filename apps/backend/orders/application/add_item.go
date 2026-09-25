package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// AddItem walks the nine steps of FND-04 §3.2. Identity is resolved before the
// transaction opens, because a re-execution would mint new identity for the
// same fact (UOW-09).
func (s Service) AddItem(ctx context.Context, cmd AddItem) (application.Outcome[domain.ItemAccepted], error) {
	var zero application.Outcome[domain.ItemAccepted]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationAddItem)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := application.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		order, stored, err := s.loadOrCreate(ctx, res, cmd.Order)
		if err != nil {
			return err
		}

		accepted, rejection := order.AddItem(domain.AddItem{
			SKU:      cmd.SKU,
			Quantity: cmd.Quantity,
			At:       domain.Instant(identity.OccurredAt),
		})
		if rejection != nil {
			// Returning nil commits a transaction with no effect, on purpose:
			// aborting would make a refusal indistinguishable from a technical
			// failure, which DEC-04 forbids (FND-04 §3.2).
			outcome = application.Rejected[domain.ItemAccepted](rejection)
			return nil
		}

		if err := res.Orders.Save(ctx, cmd.Order, order.Snapshot(), stored); err != nil {
			return err
		}
		if err := enqueueAll(ctx, res.Outbox, identity, cmd.Order, stored+1, accepted.Events()); err != nil {
			return err
		}

		outcome = application.Accepted(accepted.Response())
		return nil
	})
	if err != nil {
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		return zero, err
	}

	category := outcomeCategory(outcome)
	end(ports.Result{Outcome: category})
	instrumentation.Audit(ctx, ports.AuditEvent{
		Object:  string(cmd.Order),
		Action:  OperationAddItem,
		Outcome: category,
		At:      identity.OccurredAt,
	})
	return outcome, nil
}

// loadOrCreate is the "load or create" branch: ErrNotFound is not a failure
// here, it means the order does not exist yet and starts at version zero.
func (s Service) loadOrCreate(ctx context.Context, res Resources, id domain.OrderID) (*domain.Order, ports.Version, error) {
	snapshot, stored, err := res.Orders.Load(ctx, id)
	switch {
	case errors.Is(err, ports.ErrNotFound):
		return domain.NewOrder(id, s.ItemLimit), 0, nil
	case err != nil:
		return nil, 0, fmt.Errorf("application: add item to %s: %w", id, err)
	default:
		return domain.FromSnapshot(snapshot), stored, nil
	}
}
