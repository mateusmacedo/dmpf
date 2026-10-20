package reference_test

import (
	"context"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

const (
	cycleConsumer = "partner.commands"
	cycleKey      = ports.OutboundKey("k-cycle")
	cycleStale    = int64(100)
)

type cycleResources struct {
	ledger ports.AttemptLedger
	inbox  ports.Inbox
}

type cycleClock struct{ at ports.Instant }

func (c *cycleClock) Now() ports.Instant { return c.at }

type cycleCommand struct {
	class ports.ExternalClass
	calls int
}

func (c *cycleCommand) Submit(context.Context, ports.OutboundKey, []byte) (ports.ExternalClass, ports.Observation, error) {
	c.calls++
	return c.class, ports.Observation{}, nil
}

type cycle struct {
	store   *memory.Store
	uow     ports.UnitOfWork[cycleResources]
	ledger  memory.AttemptStore
	clock   *cycleClock
	command *cycleCommand
	steps   []application.Step
	ctx     context.Context
}

func newCycle(t *testing.T) *cycle {
	t.Helper()
	store := memory.New()
	return &cycle{
		store: store,
		uow: memory.NewUnitOfWork(store, func(tx *memory.Tx) cycleResources {
			return cycleResources{ledger: tx.AttemptLedger(), inbox: tx.Inbox(cycleConsumer)}
		}),
		ledger:  memory.NewAttemptStore(store),
		clock:   &cycleClock{at: 1_000},
		command: &cycleCommand{class: ports.ClassAccepted},
		ctx:     execctx.WithExecution(t, context.Background()),
	}
}

func (c *cycle) apply(_ context.Context, _ cycleResources, step application.ExternalStep) error {
	c.steps = append(c.steps, step.Step)
	return nil
}

func (c *cycle) profile() ports.ExternalProfile {
	return ports.ExternalProfile{
		KeySemantics: ports.KeyNative, Efficacy: ports.EfficacyOnSubmit, MaxQueries: 3,
		KeyAlphabet: "0123456789abcdef", KeyMaxLength: 32,
	}
}

func (c *cycle) dispatch(t *testing.T, message ports.MessageID) application.Disposition {
	t.Helper()
	disposition, err := application.Dispatch(c.ctx, application.ExternalDispatch[cycleResources]{
		UoW:             c.uow,
		Store:           c.ledger,
		Ledger:          func(res cycleResources) ports.AttemptLedger { return res.ledger },
		Inbox:           func(res cycleResources) ports.Inbox { return res.inbox },
		Receipt:         ports.Receipt{Consumer: cycleConsumer, MessageID: message, MessageType: "partner.Submit", PayloadHash: "h", ReceivedAt: c.clock.at},
		Command:         c.command,
		Key:             cycleKey,
		Profile:         c.profile(),
		QueryAvailable:  true,
		StaleAfter:      cycleStale,
		Clock:           c.clock,
		SettleContext:   func(ctx context.Context) (context.Context, context.CancelFunc) { return context.WithCancel(ctx) },
		Instrumentation: ports.NoInstrumentation(),
		Apply:           c.apply,
	})
	if err != nil {
		t.Fatalf("Dispatch(%s) = %v", message, err)
	}
	return disposition
}

func (c *cycle) resolve(t *testing.T, id string, finding ports.Finding) ports.ObserveResult {
	t.Helper()
	observed, err := application.Resolve(c.ctx, application.ExternalResolution[cycleResources]{
		UoW:             c.uow,
		Ledger:          func(res cycleResources) ports.AttemptLedger { return res.ledger },
		Observation:     ports.Observation{Key: cycleKey, Finding: finding, ID: id},
		Profile:         c.profile(),
		QueryAvailable:  true,
		Clock:           c.clock,
		Instrumentation: ports.NoInstrumentation(),
		Apply:           c.apply,
	})
	if err != nil {
		t.Fatalf("Resolve(%s) = %v", id, err)
	}
	return observed.Result
}

func (c *cycle) sweep(t *testing.T) []ports.Attempt {
	t.Helper()
	stale, err := c.ledger.Stale(c.ctx, c.clock.at-ports.Instant(cycleStale), ports.AttemptCursor{}, 10)
	if err != nil {
		t.Fatalf("Stale() = %v", err)
	}
	for _, attempt := range stale {
		_, err := application.Abandon(c.ctx, application.ExternalAbandon[cycleResources]{
			UoW:             c.uow,
			Ledger:          func(res cycleResources) ports.AttemptLedger { return res.ledger },
			Attempt:         attempt,
			Profile:         c.profile(),
			QueryAvailable:  true,
			Clock:           c.clock,
			Instrumentation: ports.NoInstrumentation(),
			Apply:           c.apply,
		})
		if err != nil {
			t.Fatalf("Abandon(%+v) = %v", attempt, err)
		}
	}
	return stale
}

func (c *cycle) registerProcessed(t *testing.T, message ports.MessageID) {
	t.Helper()
	err := c.uow.Within(c.ctx, func(ctx context.Context, res cycleResources) error {
		reception, err := res.inbox.Register(ctx, ports.Receipt{Consumer: cycleConsumer, MessageID: message, MessageType: "partner.Submit", PayloadHash: "h"})
		if err != nil {
			return err
		}
		return reception.Match(
			func(p ports.Pending) error { return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed}) },
			func() error { return nil }, func() error { return nil }, func() error { return nil },
		)
	})
	if err != nil {
		t.Fatalf("registering %s = %v", message, err)
	}
}

func TestTheExternalCycleAppliesEachOutcomeOnce(t *testing.T) {
	tests := []struct {
		name      string
		run       func(t *testing.T, c *cycle) []application.Disposition
		wantDisp  []application.Disposition
		wantSteps []application.Step
		wantCalls int
	}{
		{
			name: "an unanswered call is resolved once and its redelivery calls nobody",
			run: func(t *testing.T, c *cycle) []application.Disposition {
				c.command.class = ports.ClassUnknownSent
				first := c.dispatch(t, "m-a")
				redelivered := c.dispatch(t, "m-a")
				return []application.Disposition{first, redelivered}
			},
			wantDisp:  []application.Disposition{application.R1D1, application.R2},
			wantSteps: []application.Step{application.StepQuery},
			wantCalls: 1,
		},
		{
			name: "a stale attempt the sweep abandoned is not applied again by its redelivery",
			run: func(t *testing.T, c *cycle) []application.Disposition {
				if _, err := c.ledger.Begin(c.ctx, cycleKey, "m-a", c.clock.at); err != nil {
					t.Fatalf("Begin() = %v", err)
				}
				c.clock.at += ports.Instant(cycleStale)
				if abandoned := c.sweep(t); len(abandoned) != 1 {
					t.Fatalf("swept %v, want the one begun attempt", abandoned)
				}
				return []application.Disposition{c.dispatch(t, "m-a")}
			},
			wantDisp:  []application.Disposition{application.R1D1},
			wantSteps: []application.Step{application.StepQuery},
		},
		{
			name: "a processed message whose last attempt came from another message is done",
			run: func(t *testing.T, c *cycle) []application.Disposition {
				first := c.dispatch(t, "m-a")
				other := c.dispatch(t, "m-b")
				redelivered := c.dispatch(t, "m-a")
				return []application.Disposition{first, other, redelivered}
			},
			wantDisp:  []application.Disposition{application.R1D1, application.R1D1, application.R2},
			wantSteps: []application.Step{application.StepConfirm, application.StepConfirm},
			wantCalls: 2,
		},
		{
			name: "a redelivery the inbox already settled leaves no begun attempt for the sweep",
			run: func(t *testing.T, c *cycle) []application.Disposition {
				c.registerProcessed(t, "m-a")
				disposition := c.dispatch(t, "m-a")
				c.clock.at += ports.Instant(cycleStale)
				if swept := c.sweep(t); len(swept) != 0 {
					t.Fatalf("swept %v, want no begun attempt left", swept)
				}
				return []application.Disposition{disposition}
			},
			wantDisp:  []application.Disposition{application.R2},
			wantSteps: []application.Step{},
			wantCalls: 1,
		},
		{
			name: "an observation confirms once and its duplicate applies nothing",
			run: func(t *testing.T, c *cycle) []application.Disposition {
				c.command.class = ports.ClassUnknownSent
				disposition := c.dispatch(t, "m-a")
				if got := []ports.ObserveResult{c.resolve(t, "o-1", ports.FindingApplied), c.resolve(t, "o-1", ports.FindingApplied)}; !slices.Equal(got, []ports.ObserveResult{ports.ObserveRecorded, ports.ObserveDuplicate}) {
					t.Fatalf("resolutions = %v, want recorded then duplicate", got)
				}
				return []application.Disposition{disposition}
			},
			wantDisp:  []application.Disposition{application.R1D1},
			wantSteps: []application.Step{application.StepQuery, application.StepConfirm},
			wantCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCycle(t)

			got := tt.run(t, c)

			if !slices.Equal(got, tt.wantDisp) {
				t.Errorf("dispositions = %v, want %v", got, tt.wantDisp)
			}
			if !slices.Equal(append([]application.Step{}, c.steps...), tt.wantSteps) {
				t.Errorf("applied steps = %v, want %v", c.steps, tt.wantSteps)
			}
			if c.command.calls != tt.wantCalls {
				t.Errorf("partner calls = %d, want %d", c.command.calls, tt.wantCalls)
			}
		})
	}
}
