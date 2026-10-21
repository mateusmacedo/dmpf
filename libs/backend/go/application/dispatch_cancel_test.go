package application_test

import (
	"context"
	"errors"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type blockingCommand struct {
	inner   *extCommand
	entered chan struct{}
}

func (c *blockingCommand) Submit(ctx context.Context, key ports.OutboundKey, payload []byte) (ports.ExternalClass, ports.Observation, error) {
	close(c.entered)
	<-ctx.Done()
	return c.inner.Submit(ctx, key, payload)
}

type dispatchResult struct {
	disposition usecase.Disposition
	err         error
}

func dispatchCancelledMidSubmit(t *testing.T, rig *dispatchRig, queryAvailable bool) (usecase.Disposition, error) {
	t.Helper()
	command := &blockingCommand{inner: rig.command, entered: make(chan struct{})}
	d := rig.dispatch()
	d.Command, d.QueryAvailable = command, queryAvailable

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan dispatchResult, 1)
	go func() {
		disposition, err := usecase.Dispatch(ctx, d)
		done <- dispatchResult{disposition, err}
	}()
	<-command.entered
	cancel()
	result := <-done
	return result.disposition, result.err
}

func assertSettledUncancelled(t *testing.T, rig *dispatchRig) {
	t.Helper()
	for _, err := range append(rig.store.finishErrs, rig.tx.finishErrs...) {
		if err != nil {
			t.Errorf("the outcome was recorded under a cancelled context: %v", err)
		}
	}
}

func TestDispatchSettlesAnUnknownSentWhenTheHandlerIsRevokedMidSubmit(t *testing.T) {
	tests := []struct {
		name           string
		queryAvailable bool
		step           usecase.Step
	}{
		{"with a query port it goes to the query", true, usecase.StepQuery},
		{"without a query port it goes to the reconciliation", false, usecase.StepReconcile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newDispatchRig()
			rig.command.class, rig.command.err = ports.ClassUnknownSent, context.Canceled
			rig.tx.finished = ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassUnknownSent}

			disposition, err := dispatchCancelledMidSubmit(t, rig, tt.queryAvailable)

			assertDispatch(t, rig, disposition, err, dispatchWant{
				disposition: usecase.R1D1, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassUnknownSent},
				steps: []usecase.Step{tt.step}, marked: ports.ClassUnknownSent, completed: true,
			})
			assertSettledUncancelled(t, rig)
			if len(rig.inbox.receipts) != 1 || rig.inbox.receipts[0].MessageID != dispatchMsg {
				t.Errorf("receipts = %+v, want the dispatch receipt once", rig.inbox.receipts)
			}
		})
	}
}

func TestDispatchFinishesAnInterruptedDialThroughTheStoreWhenRevokedMidSubmit(t *testing.T) {
	rig := newDispatchRig()
	rig.command.class, rig.command.err = ports.ClassRetryableTransport, context.Canceled

	disposition, err := dispatchCancelledMidSubmit(t, rig, true)

	assertDispatch(t, rig, disposition, err, dispatchWant{
		disposition: usecase.R1D3, err: context.Canceled, category: usecase.TransientDependency, submits: 1,
		storeFinishes: []ports.ExternalClass{ports.ClassRetryableTransport}, txFinishes: []ports.ExternalClass{},
		steps: []usecase.Step{}, marked: ports.ClassRetryableTransport,
	})
	assertSettledUncancelled(t, rig)
	if len(rig.inbox.receipts) != 0 {
		t.Errorf("receipts = %+v, want the inbox untouched", rig.inbox.receipts)
	}
	var failure *usecase.Failure
	if !errors.As(err, &failure) || !failure.Retryable() {
		t.Errorf("err = %v, want a retryable failure", err)
	}
}
