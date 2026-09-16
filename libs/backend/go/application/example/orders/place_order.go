package ordersapp

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// PlaceOrder walks the same nine steps as AddItem, but only loads: an absent
// aggregate comes back as a wrapped technical error, not a rejection, because
// no UPR produced one and the edge category belongs to FND-07.
func (s Service) PlaceOrder(ctx context.Context, cmd PlaceOrder) (application.Outcome[orders.PlacedResponse], error) {
	var zero application.Outcome[orders.PlacedResponse]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationPlaceOrder)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(authorizationResult(err))
		return zero, err
	}

	identity := application.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		snapshot, stored, err := res.Orders.Load(ctx, cmd.Order)
		if err != nil {
			return fmt.Errorf("ordersapp: place order %s: %w", cmd.Order, err)
		}

		order := orders.FromSnapshot(snapshot)
		accepted, rejection := order.Place(orders.PlaceOrder{At: orders.Instant(identity.OccurredAt.Unix())})
		if rejection != nil {
			outcome = application.Rejected[orders.PlacedResponse](rejection)
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
		Action:  OperationPlaceOrder,
		Outcome: category,
		At:      identity.OccurredAt,
	})
	return outcome, nil
}
