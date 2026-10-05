package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// AddItem walks the nine steps of FND-04 §3.2. Identity is resolved before the
// transaction opens, because a re-execution would mint new identity for the
// same fact (UOW-09).
func (s Service) AddItem(ctx context.Context, cmd AddItem) (usecase.Outcome[domain.ItemAccepted], error) {
	var zero usecase.Outcome[domain.ItemAccepted]

	instrumentation := s.instrumentation()
	ctx, end := instrumentation.BeginOperation(ctx, OperationAddItem)

	if err := s.Authorize(ctx, cmd); err != nil {
		end(ports.AuthorizationResult(err))
		return zero, err
	}

	identity := usecase.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)
	fingerprint := usecase.NewFingerprint(OperationAddItem).
		String(string(cmd.Order)).String(string(cmd.SKU)).Int(int64(cmd.Quantity))

	outcome, replayed := zero, false
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		var err error
		outcome, replayed, err = idempotent(ctx, s, res, fingerprint, OperationAddItem, identity.OccurredAt, itemAcceptedCodec,
			func() (usecase.Outcome[domain.ItemAccepted], error) { return s.addItem(ctx, res, cmd, identity) })
		return err
	})
	if err != nil {
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		return zero, err
	}

	category := outcome.Category()
	end(ports.Result{Outcome: category})
	if !replayed {
		instrumentation.Audit(ctx, ports.AuditEvent{
			Object:  string(cmd.Order),
			Action:  OperationAddItem,
			Outcome: category,
			At:      identity.OccurredAt,
		})
	}
	return outcome, nil
}

func (s Service) addItem(ctx context.Context, res Resources, cmd AddItem, identity usecase.Identity) (usecase.Outcome[domain.ItemAccepted], error) {
	var zero usecase.Outcome[domain.ItemAccepted]

	order, stored, err := s.loadOrCreate(ctx, res, cmd.Order)
	if err != nil {
		return zero, err
	}

	accepted, rejection := order.AddItem(domain.AddItem{
		SKU:      cmd.SKU,
		Quantity: cmd.Quantity,
		At:       domain.Instant(identity.OccurredAt),
	})
	if rejection != nil {
		// A refusal returns no error, on purpose: the transaction commits it
		// with no business effect, and aborting would make it indistinguishable
		// from a technical failure, which DEC-04 forbids (FND-04 §3.2).
		return usecase.Rejected[domain.ItemAccepted](rejection), nil
	}

	if err := res.Orders.Save(ctx, cmd.Order, order.Snapshot(), stored); err != nil {
		return zero, err
	}
	if err := usecase.Enqueue(ctx, res.Outbox, identity, origin(cmd.Order), stored+1, accepted.Events()); err != nil {
		return zero, err
	}
	return usecase.Accepted(accepted.Response()), nil
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
