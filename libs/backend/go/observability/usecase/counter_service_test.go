package usecase_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// counterService is the fixture use case of this suite: the nine steps of
// FND-04 §3.2 over the smallest aggregate with both branches, calling the hook
// exactly where a real service does, so what the hook emits is proved end to end.

const (
	operationBump = "counters.Bump"
	operationFind = "counters.Find"
)

type counterID string

type counterState struct {
	ID    counterID
	Total int
	Limit int
}

type bumpCounter struct {
	Counter counterID
	By      int
}

type bumpResponse struct {
	Counter counterID
	Total   int
}

type counterBumped struct {
	Counter counterID
	By      int
	Total   int
}

func (counterBumped) EventName() string { return "counters.bumped" }

const codeLimitExceeded domain.Code = "counters/limit-exceeded"

var countersTable = memory.Table[counterID, counterState]{Name: "counters"}

type counterResources struct {
	Counters ports.Repository[counterID, counterState]
	Outbox   ports.Outbox
}

type counterService struct {
	UoW       ports.UnitOfWork[counterResources]
	Reader    ports.Reader[counterID, counterState]
	Clock     ports.Clock
	IDs       ports.IDGenerator
	Authorize application.AuthorizeWithContext[bumpCounter]
	Limit     int

	Instrumentation ports.Instrumentation
}

func (s counterService) Bump(ctx context.Context, execution ports.ExecutionContext, cmd bumpCounter) (application.Outcome[bumpResponse], error) {
	var zero application.Outcome[bumpResponse]

	ctx, end := s.Instrumentation.BeginOperation(ctx, operationBump)
	if err := s.Authorize(ctx, execution, cmd); err != nil {
		if errors.Is(err, ports.ErrDenied) {
			end(ports.Result{Outcome: ports.OutcomeDenied})
		} else {
			end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		}
		return zero, err
	}
	identity := application.ResolveIdentity(s.Clock, s.IDs, 1)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res counterResources) error {
		state, stored, err := res.Counters.Load(ctx, cmd.Counter)
		switch {
		case errors.Is(err, ports.ErrNotFound):
			state = counterState{ID: cmd.Counter, Limit: s.Limit}
		case err != nil:
			return err
		}

		attempted := state.Total + cmd.By
		if attempted > state.Limit {
			outcome = application.Rejected[bumpResponse](domain.Reject(codeLimitExceeded, "the counter would exceed its limit"))
			return nil
		}
		state.Total = attempted

		if err := res.Counters.Save(ctx, cmd.Counter, state, stored); err != nil {
			return err
		}
		entry := ports.OutboxEntry{
			MessageID:        identity.MessageIDs[0],
			OccurredAt:       identity.OccurredAt,
			Intent:           ports.PublishIntent{Destination: "counters.events", PartitionKey: string(cmd.Counter)},
			AggregateType:    "counters.Counter",
			AggregateID:      string(cmd.Counter),
			AggregateVersion: stored + 1,
			Event:            counterBumped{Counter: cmd.Counter, By: cmd.By, Total: state.Total},
			Context:          application.MessageContextFor(ctx, identity.MessageIDs[0]),
		}
		if err := res.Outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
		outcome = application.Accepted(bumpResponse{Counter: cmd.Counter, Total: state.Total})
		return nil
	})
	if err != nil {
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		return zero, err
	}

	category := ports.OutcomeAccepted
	if _, refused := outcome.Rejection(); refused {
		category = ports.OutcomeRejected
	}
	end(ports.Result{Outcome: category})
	s.Instrumentation.Audit(ctx, ports.AuditEvent{
		Object:  string(cmd.Counter),
		Action:  operationBump,
		Outcome: category,
		At:      identity.OccurredAt,
	})
	return outcome, nil
}

func (s counterService) Find(ctx context.Context, id counterID) (counterState, error) {
	ctx, end := s.Instrumentation.BeginOperation(ctx, operationFind)

	state, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		failed := fmt.Errorf("find counter %s: %w", id, err)
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: failed})
		return counterState{}, failed
	}

	end(ports.Result{Outcome: ports.OutcomeAccepted})
	return state, nil
}
