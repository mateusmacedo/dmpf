package reservationsapp

import (
	"context"
	"errors"
	"fmt"

	dmpfapplication "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application"
	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/reservations"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// Consume walks the seven steps of FND-04 §6.3. Step 5 is the only branch
// point (INB-11): R2, R3 and R4 short-circuit without writing, and only R1
// reaches axis 2. The broker effect of §6.4 is the adapter's, not this
// method's (INB-08).
func (s Service) Consume(ctx context.Context, cmd ConsumeOrderPlaced) (dmpfapplication.Disposition, error) {
	if err := s.Authorize(ctx, cmd); err != nil {
		return dmpfapplication.Classify(err), err
	}

	identity := dmpfapplication.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)

	var disposition dmpfapplication.Disposition
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		reception, err := res.Inbox.Register(ctx, dmpfports.Receipt{
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
			func(p dmpfports.Pending) error {
				return s.consumeFirst(ctx, res, cmd, identity, p, &disposition)
			},
			func() error { disposition = dmpfapplication.R2; return nil },
			func() error { disposition = dmpfapplication.R3; return nil },
			func() error { disposition = dmpfapplication.R4; return nil },
		)
	})
	if err != nil {
		return dmpfapplication.Classify(err), err
	}
	return disposition, nil
}

// consumeFirst is axis 2 of §6.4, reachable only under R1: it decides D1,
// D2 or a technical failure and completes the pending reception accordingly.
func (s Service) consumeFirst(
	ctx context.Context,
	res Resources,
	cmd ConsumeOrderPlaced,
	identity dmpfapplication.Identity,
	pending dmpfports.Pending,
	disposition *dmpfapplication.Disposition,
) error {
	snapshot, stored, err := res.Reservations.Load(ctx, cmd.Order)
	var reservation *reservations.Reservation
	switch {
	case errors.Is(err, dmpfports.ErrNotFound):
		reservation, stored = reservations.NewReservation(cmd.Order), 0
	case err != nil:
		return fmt.Errorf("reservationsapp: consume %s: %w", cmd.Order, err)
	default:
		reservation = reservations.FromSnapshot(snapshot)
	}

	accepted, rejection := reservation.Reserve(reservations.Reserve{
		Items: cmd.Items,
		At:    reservations.Instant(identity.OccurredAt.Unix()),
	})
	if rejection != nil {
		*disposition = dmpfapplication.R1D2
		return pending.Complete(ctx, dmpfports.Completion{
			Status:    dmpfports.StatusRejected,
			At:        identity.OccurredAt,
			LastError: string(rejection.Code()),
		})
	}

	if err := res.Reservations.Save(ctx, cmd.Order, reservation.Snapshot(), stored); err != nil {
		if errors.Is(err, dmpfports.ErrVersionConflict) {
			// MAP-07: the predicate is declared here — a reread replays the
			// decision instead of repeating one already taken.
			return dmpfapplication.NewFailure(dmpfapplication.Conflict, true, err)
		}
		return err
	}
	if err := enqueueAll(ctx, res.Outbox, identity, cmd.Order, stored+1, accepted.Events()); err != nil {
		return err
	}

	*disposition = dmpfapplication.R1D1
	return pending.Complete(ctx, dmpfports.Completion{Status: dmpfports.StatusProcessed, At: identity.OccurredAt})
}

// enqueueAll authors the seven fields the application service owns (FND-04
// §2.3, BLK-04, BLK-05). ordersapp.enqueueAll is not exported, so this
// realization repeats the same shape locally rather than widening its API.
func enqueueAll(
	ctx context.Context,
	outbox dmpfports.Outbox,
	identity dmpfapplication.Identity,
	order reservations.OrderID,
	written dmpfports.Version,
	events []dmpfdomain.DomainEvent,
) error {
	if len(events) > len(identity.MessageIDs) {
		panic(fmt.Sprintf(
			"reservationsapp: the decision produced %d events but only %d identifiers were resolved; raise maxEventsPerCommand",
			len(events), len(identity.MessageIDs)))
	}

	for i, event := range events {
		entry := dmpfports.OutboxEntry{
			MessageID:        identity.MessageIDs[i],
			OccurredAt:       identity.OccurredAt,
			Intent:           dmpfports.PublishIntent{Destination: Destination, PartitionKey: string(order)},
			AggregateType:    AggregateType,
			AggregateID:      string(order),
			AggregateVersion: written,
			Event:            event,
		}
		if err := outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}
