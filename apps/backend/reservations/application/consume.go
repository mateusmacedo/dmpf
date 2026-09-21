package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

// Consume walks the seven steps of FND-04 §6.3. Step 5 is the only branch
// point (INB-11): R2, R3 and R4 short-circuit without writing, and only R1
// reaches axis 2. The broker effect of §6.4 is the adapter's, not this
// method's (INB-08).
func (s Service) Consume(ctx context.Context, execution ports.ExecutionContext, cmd ConsumeOrderPlaced) (application.Disposition, error) {
	if err := s.Authorize(ctx, execution, cmd); err != nil {
		return application.Classify(err), err
	}

	identity := application.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	var disposition application.Disposition
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		reception, err := res.Inbox.Register(ctx, ports.Receipt{
			Consumer:    s.Consumer,
			MessageID:   cmd.MessageID,
			MessageType: cmd.MessageType,
			PayloadHash: cmd.PayloadHash,
			ReceivedAt:  cmd.ReceivedAt,
		})
		if err != nil {
			return err
		}
		return reception.Match(
			func(p ports.Pending) error {
				return s.consumeFirst(ctx, res, cmd, identity, p, &disposition)
			},
			func() error { disposition = application.R2; return nil },
			func() error { disposition = application.R3; return nil },
			func() error { disposition = application.R4; return nil },
		)
	})
	if err != nil {
		return application.Classify(err), err
	}
	return disposition, nil
}

// consumeFirst is axis 2 of §6.4, reachable only under R1: it decides D1,
// D2 or a technical failure and completes the pending reception accordingly.
func (s Service) consumeFirst(
	ctx context.Context,
	res Resources,
	cmd ConsumeOrderPlaced,
	identity application.Identity,
	pending ports.Pending,
	disposition *application.Disposition,
) error {
	snapshot, stored, err := res.Reservations.Load(ctx, cmd.Order)
	var reservation *domain.Reservation
	switch {
	case errors.Is(err, ports.ErrNotFound):
		reservation, stored = domain.NewReservation(cmd.Order), 0
	case err != nil:
		return fmt.Errorf("application: consume %s: %w", cmd.Order, err)
	default:
		reservation = domain.FromSnapshot(snapshot)
	}

	accepted, rejection := reservation.Reserve(domain.Reserve{
		Items: cmd.Items,
		At:    domain.Instant(identity.OccurredAt.Unix()),
	})
	if rejection != nil {
		*disposition = application.R1D2
		return pending.Complete(ctx, ports.Completion{
			Status:    ports.StatusRejected,
			At:        identity.OccurredAt,
			LastError: string(rejection.Code()),
		})
	}

	if err := res.Reservations.Save(ctx, cmd.Order, reservation.Snapshot(), stored); err != nil {
		if errors.Is(err, ports.ErrVersionConflict) {
			// MAP-07: the predicate is declared here — a reread replays the
			// decision instead of repeating one already taken.
			return application.NewFailure(application.Conflict, true, err)
		}
		return err
	}
	if err := enqueueAll(ctx, res.Outbox, identity, cmd.Order, stored+1, accepted.Events()); err != nil {
		return err
	}

	*disposition = application.R1D1
	return pending.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: identity.OccurredAt})
}

// enqueueAll authors the seven fields the application service owns (FND-04
// §2.3, BLK-04, BLK-05).
func enqueueAll(
	ctx context.Context,
	outbox ports.Outbox,
	identity application.Identity,
	order domain.OrderID,
	written ports.Version,
	events []kernel.DomainEvent,
) error {
	if len(events) > len(identity.MessageIDs) {
		panic(fmt.Sprintf(
			"application: the decision produced %d events but only %d identifiers were resolved; raise maxEventsPerCommand",
			len(events), len(identity.MessageIDs)))
	}

	for i, event := range events {
		entry := ports.OutboxEntry{
			MessageID:        identity.MessageIDs[i],
			OccurredAt:       identity.OccurredAt,
			Intent:           ports.PublishIntent{Destination: Destination, PartitionKey: string(order)},
			AggregateType:    AggregateType,
			AggregateID:      string(order),
			AggregateVersion: written,
			Event:            event,
			Context:          application.MessageContextFor(ctx, identity.MessageIDs[i]),
		}
		if err := outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}
