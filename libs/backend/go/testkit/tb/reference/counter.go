package reference

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	OperationBump = "counters.Bump"
	OperationFind = "counters.Find"
)

type CounterID string

type CounterState struct {
	ID    CounterID
	Total int
	Limit int
}

type BumpCounter struct {
	Counter CounterID
	By      int
}

type BumpResponse struct {
	Counter CounterID
	Total   int
}

type CounterBumped struct {
	Counter CounterID
	By      int
	Total   int
}

func (CounterBumped) EventName() string { return "counters.bumped" }

const CodeLimitExceeded domain.Code = "counters/limit-exceeded"

var CountersTable = memory.Table[CounterID, CounterState]{Name: "counters"}

type CounterResources struct {
	Counters ports.Repository[CounterID, CounterState]
	Outbox   ports.Outbox
}

// CounterService walks the nine steps of FND-04 §3.2 over the smallest aggregate
// with both branches, calling the instrumentation hook where a real service does.
type CounterService struct {
	UoW       ports.UnitOfWork[CounterResources]
	Reader    ports.Reader[CounterID, CounterState]
	Clock     ports.Clock
	IDs       ports.IDGenerator
	Authorize application.Authorize[BumpCounter]
	Limit     int

	Instrumentation ports.Instrumentation
}

func (s CounterService) instrumentation() ports.Instrumentation {
	if s.Instrumentation == nil {
		return ports.NoInstrumentation()
	}
	return s.Instrumentation
}

func (s CounterService) Bump(ctx context.Context, cmd BumpCounter) (application.Outcome[BumpResponse], error) {
	var zero application.Outcome[BumpResponse]

	ctx, end := s.instrumentation().BeginOperation(ctx, OperationBump)
	if err := s.Authorize(ctx, cmd); err != nil {
		if errors.Is(err, ports.ErrDenied) {
			end(ports.Result{Outcome: ports.OutcomeDenied})
		} else {
			end(ports.Result{Outcome: ports.OutcomeFailed, Err: err})
		}
		return zero, err
	}
	identity := application.ResolveIdentity(s.Clock, s.IDs, 1)

	outcome := zero
	err := s.UoW.Within(ctx, func(ctx context.Context, res CounterResources) error {
		state, stored, err := res.Counters.Load(ctx, cmd.Counter)
		switch {
		case errors.Is(err, ports.ErrNotFound):
			state = CounterState{ID: cmd.Counter, Limit: s.Limit}
		case err != nil:
			return err
		}

		attempted := state.Total + cmd.By
		if attempted > state.Limit {
			outcome = application.Rejected[BumpResponse](domain.Reject(CodeLimitExceeded, "the counter would exceed its limit"))
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
			Event:            CounterBumped{Counter: cmd.Counter, By: cmd.By, Total: state.Total},
			Context:          application.MessageContextFor(ctx, identity.MessageIDs[0]),
		}
		if err := res.Outbox.Enqueue(ctx, entry); err != nil {
			return err
		}
		outcome = application.Accepted(BumpResponse{Counter: cmd.Counter, Total: state.Total})
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
	s.instrumentation().Audit(ctx, ports.AuditEvent{
		Object:  string(cmd.Counter),
		Action:  OperationBump,
		Outcome: category,
		At:      identity.OccurredAt,
	})
	return outcome, nil
}

func (s CounterService) Find(ctx context.Context, id CounterID) (CounterState, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFind)

	state, _, err := s.Reader.Load(ctx, id)
	if err != nil {
		failed := fmt.Errorf("find counter %s: %w", id, err)
		end(ports.Result{Outcome: ports.OutcomeFailed, Err: failed})
		return CounterState{}, failed
	}

	end(ports.Result{Outcome: ports.OutcomeAccepted})
	return state, nil
}
