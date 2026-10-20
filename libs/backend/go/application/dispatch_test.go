package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	dispatchKey   = ports.OutboundKey("k-out")
	dispatchMsg   = ports.MessageID("m-1")
	staleAfter    = int64(100)
	attemptOpened = ports.Instant(1_000)
)

var (
	errSubmit = errors.New("dispatch_test: partner unreachable")
	errLedger = errors.New("dispatch_test: ledger failed")
	errApply  = errors.New("dispatch_test: apply failed")
	errCommit = errors.New("dispatch_test: commit failed")
)

func openedAttempt(n int) ports.Attempt {
	return ports.Attempt{Tenant: "acme", Key: dispatchKey, N: n, MessageID: dispatchMsg, StartedAt: attemptOpened}
}

type dispatchRig struct {
	store   *extLedger
	tx      *extLedger
	inbox   *fakeInbox
	pending *fakePending
	uow     *extUoW
	command *extCommand
	apply   *extApply
	clock   *countingClock
	inst    *extInstrumentation
}

func newDispatchRig() *dispatchRig {
	inbox, pending := firstInbox()
	tx := &extLedger{finished: ports.Finished{Result: ports.FinishRecorded}}
	uow := &extUoW{res: extResources{ledger: tx, inbox: inbox}}
	return &dispatchRig{
		store:   &extLedger{start: ports.StartedAttempt(openedAttempt(1)), finished: ports.Finished{Result: ports.FinishRecorded}},
		tx:      tx,
		inbox:   inbox,
		pending: pending,
		uow:     uow,
		command: &extCommand{class: ports.ClassAccepted, uow: uow},
		apply:   &extApply{uow: uow},
		clock:   &countingClock{at: attemptOpened + 10},
		inst:    &extInstrumentation{},
	}
}

func (r *dispatchRig) dispatch() usecase.ExternalDispatch[extResources] {
	return usecase.ExternalDispatch[extResources]{
		UoW:             r.uow,
		Store:           r.store,
		Ledger:          func(res extResources) ports.AttemptLedger { return res.ledger },
		Inbox:           func(res extResources) ports.Inbox { return res.inbox },
		Receipt:         ports.Receipt{Consumer: "ctx.commands", MessageID: dispatchMsg, MessageType: "order.placed"},
		Command:         r.command,
		Key:             dispatchKey,
		Payload:         []byte("payload"),
		Profile:         validProfile(),
		QueryAvailable:  true,
		StaleAfter:      staleAfter,
		Clock:           r.clock,
		SettleContext:   settleWithin,
		Instrumentation: r.inst,
		Apply:           r.apply.apply,
	}
}

type dispatchWant struct {
	disposition   usecase.Disposition
	err           error
	category      usecase.Category
	submits       int
	withins       int
	storeFinishes []ports.ExternalClass
	txFinishes    []ports.ExternalClass
	steps         []usecase.Step
	marked        ports.ExternalClass
	conflict      bool
	presumed      bool
	completed     bool
}

func TestDispatch(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(r *dispatchRig)
		want    dispatchWant
	}{
		{
			name: "a pending attempt inside the cutoff is a retryable failure that calls nobody",
			arrange: func(r *dispatchRig) {
				r.store.start = ports.PendingAttempt(openedAttempt(1))
				r.clock.at = attemptOpened + ports.Instant(staleAfter) - 1
			},
			want: dispatchWant{
				disposition: usecase.R1D3, err: ports.ErrAttemptInFlight, category: usecase.TransientDependency,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{}, steps: []usecase.Step{},
			},
		},
		{
			name: "a pending attempt at the cutoff is closed as unknown sent without a call",
			arrange: func(r *dispatchRig) {
				r.store.start = ports.PendingAttempt(openedAttempt(1))
				r.clock.at = attemptOpened + ports.Instant(staleAfter)
			},
			want: dispatchWant{
				disposition: usecase.R1D1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassUnknownSent},
				steps: []usecase.Step{usecase.StepQuery}, marked: ports.ClassUnknownSent, presumed: true, completed: true,
			},
		},
		{
			name: "a done attempt goes to the transaction with the recorded class and applies nothing again",
			arrange: func(r *dispatchRig) {
				r.store.start = ports.DoneAttempt(openedAttempt(1), ports.ClassFinalBusiness)
				r.tx.finished = ports.Finished{Result: ports.FinishUnchanged, Recorded: ports.ClassFinalBusiness}
			},
			want: dispatchWant{
				disposition: usecase.R1D1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassFinalBusiness},
				steps: []usecase.Step{}, marked: ports.ClassFinalBusiness, completed: true,
			},
		},
		{
			name: "an accepted submission is recorded and confirmed in the transaction",
			arrange: func(r *dispatchRig) {
				r.tx.finished = ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassAccepted}
			},
			want: dispatchWant{
				disposition: usecase.R1D1, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassAccepted},
				steps: []usecase.Step{usecase.StepConfirm}, marked: ports.ClassAccepted, completed: true,
			},
		},
		{
			name: "a final rejection is recorded and failed in the transaction",
			arrange: func(r *dispatchRig) {
				r.command.class = ports.ClassFinalBusiness
				r.tx.finished = ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassFinalBusiness}
			},
			want: dispatchWant{
				disposition: usecase.R1D1, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassFinalBusiness},
				steps: []usecase.Step{usecase.StepFail}, marked: ports.ClassFinalBusiness, completed: true,
			},
		},
		{
			name: "the finding the submission carried prevails over the class",
			arrange: func(r *dispatchRig) {
				r.command.class = ports.ClassUnknownSent
				r.command.obs = ports.Observation{Key: dispatchKey, Finding: ports.FindingApplied, ID: "o-1"}
				r.tx.finished = ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassUnknownSent}
			},
			want: dispatchWant{
				disposition: usecase.R1D1, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassUnknownSent},
				steps: []usecase.Step{usecase.StepConfirm}, marked: ports.ClassUnknownSent, completed: true,
			},
		},
		{
			name: "an unanswered submission goes to the query",
			arrange: func(r *dispatchRig) {
				r.command.class, r.command.err = ports.ClassUnknownSent, errSubmit
				r.tx.finished = ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassUnknownSent}
			},
			want: dispatchWant{
				disposition: usecase.R1D1, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassUnknownSent},
				steps: []usecase.Step{usecase.StepQuery}, marked: ports.ClassUnknownSent, completed: true,
			},
		},
		{
			name: "a submission with no class is recorded as ambiguous",
			arrange: func(r *dispatchRig) {
				r.command.class = 0
				r.tx.finished = ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassUnknownAmbiguous}
			},
			want: dispatchWant{
				disposition: usecase.R1D1, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassUnknownAmbiguous},
				steps: []usecase.Step{usecase.StepQuery}, marked: ports.ClassUnknownAmbiguous, completed: true,
			},
		},
		{
			name: "a spent transport budget finishes through the store and fails retryably without the inbox",
			arrange: func(r *dispatchRig) {
				r.command.class, r.command.err = ports.ClassRetryableTransport, errSubmit
			},
			want: dispatchWant{
				disposition: usecase.R1D3, err: errSubmit, category: usecase.TransientDependency, submits: 1,
				storeFinishes: []ports.ExternalClass{ports.ClassRetryableTransport}, txFinishes: []ports.ExternalClass{},
				steps: []usecase.Step{}, marked: ports.ClassRetryableTransport,
			},
		},
		{
			name: "a spent upstream budget finishes through the store and fails retryably without the inbox",
			arrange: func(r *dispatchRig) {
				r.command.class = ports.ClassRetryableUpstream
			},
			want: dispatchWant{
				disposition: usecase.R1D3, category: usecase.TransientDependency, submits: 1,
				storeFinishes: []ports.ExternalClass{ports.ClassRetryableUpstream}, txFinishes: []ports.ExternalClass{},
				steps: []usecase.Step{}, marked: ports.ClassRetryableUpstream,
			},
		},
		{
			name: "a spent budget on an attempt already closed settles on the recorded class without applying",
			arrange: func(r *dispatchRig) {
				r.command.class = ports.ClassRetryableUpstream
				r.store.finished = ports.Finished{Result: ports.FinishConflict, Recorded: ports.ClassUnknownSent}
				r.tx.finished = ports.Finished{Result: ports.FinishUnchanged, Recorded: ports.ClassUnknownSent}
			},
			want: dispatchWant{
				disposition: usecase.R1D1, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{ports.ClassRetryableUpstream}, txFinishes: []ports.ExternalClass{ports.ClassUnknownSent},
				steps: []usecase.Step{}, marked: ports.ClassUnknownSent, conflict: true, completed: true,
			},
		},
		{
			name: "a spent budget the store already holds as retryable stays a retry",
			arrange: func(r *dispatchRig) {
				r.command.class = ports.ClassRetryableTransport
				r.store.finished = ports.Finished{Result: ports.FinishUnchanged, Recorded: ports.ClassRetryableTransport}
			},
			want: dispatchWant{
				disposition: usecase.R1D3, category: usecase.TransientDependency, submits: 1,
				storeFinishes: []ports.ExternalClass{ports.ClassRetryableTransport}, txFinishes: []ports.ExternalClass{},
				steps: []usecase.Step{}, marked: ports.ClassRetryableTransport,
			},
		},
		{
			name: "a late class after a closed attempt is a conflict that applies nothing and follows the recorded one",
			arrange: func(r *dispatchRig) {
				r.tx.finished = ports.Finished{Result: ports.FinishConflict, Recorded: ports.ClassUnknownSent}
			},
			want: dispatchWant{
				disposition: usecase.R1D1, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassAccepted},
				steps: []usecase.Step{}, marked: ports.ClassUnknownSent, conflict: true, completed: true,
			},
		},
		{
			name:    "a processed redelivery short-circuits as R2",
			arrange: func(r *dispatchRig) { r.inbox.reception = ports.ProcessedReception() },
			want: dispatchWant{
				disposition: usecase.R2, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassAccepted},
				steps: []usecase.Step{}, marked: ports.ClassAccepted,
			},
		},
		{
			name:    "a rejected redelivery short-circuits as R3",
			arrange: func(r *dispatchRig) { r.inbox.reception = ports.RejectedReception() },
			want: dispatchWant{
				disposition: usecase.R3, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassAccepted},
				steps: []usecase.Step{}, marked: ports.ClassAccepted,
			},
		},
		{
			name:    "a colliding redelivery short-circuits as R4",
			arrange: func(r *dispatchRig) { r.inbox.reception = ports.CollisionReception() },
			want: dispatchWant{
				disposition: usecase.R4, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassAccepted},
				steps: []usecase.Step{}, marked: ports.ClassAccepted,
			},
		},
		{
			name: "a processed redelivery of a done attempt records nothing",
			arrange: func(r *dispatchRig) {
				r.store.start = ports.DoneAttempt(openedAttempt(1), ports.ClassAccepted)
				r.inbox.reception = ports.ProcessedReception()
			},
			want: dispatchWant{
				disposition: usecase.R2, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{},
				steps: []usecase.Step{}, marked: ports.ClassAccepted,
			},
		},
		{
			name: "a processed redelivery past the cutoff closes the presumed attempt it took over",
			arrange: func(r *dispatchRig) {
				r.store.start = ports.PendingAttempt(openedAttempt(1))
				r.clock.at = attemptOpened + ports.Instant(staleAfter)
				r.inbox.reception = ports.ProcessedReception()
			},
			want: dispatchWant{
				disposition: usecase.R2, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassUnknownSent},
				steps: []usecase.Step{}, marked: ports.ClassUnknownSent, presumed: true,
			},
		},
		{
			name:    "a failed close of a redelivery is returned as it is",
			arrange: func(r *dispatchRig) { r.inbox.reception = ports.ProcessedReception(); r.tx.finishErr = errLedger },
			want: dispatchWant{
				disposition: usecase.R1D4, err: errLedger, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassAccepted},
				steps: []usecase.Step{}, marked: ports.ClassAccepted,
			},
		},
		{
			name:    "a failed begin calls nobody",
			arrange: func(r *dispatchRig) { r.store.beginErr = errLedger },
			want: dispatchWant{
				disposition: usecase.R1D4, err: errLedger,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{}, steps: []usecase.Step{},
			},
		},
		{
			name: "a failed store finish is returned as it is",
			arrange: func(r *dispatchRig) {
				r.command.class = ports.ClassRetryableTransport
				r.store.finishErr = errLedger
			},
			want: dispatchWant{
				disposition: usecase.R1D4, err: errLedger, submits: 1,
				storeFinishes: []ports.ExternalClass{ports.ClassRetryableTransport}, txFinishes: []ports.ExternalClass{},
				steps: []usecase.Step{}, marked: ports.ClassRetryableTransport,
			},
		},
		{
			name:    "a register timeout is a retryable disposition",
			arrange: func(r *dispatchRig) { r.inbox.err = ports.ErrRegisterTimeout },
			want: dispatchWant{
				disposition: usecase.R1D3, err: ports.ErrRegisterTimeout, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{},
				steps: []usecase.Step{}, marked: ports.ClassAccepted,
			},
		},
		{
			name:    "a failed transactional finish applies nothing",
			arrange: func(r *dispatchRig) { r.tx.finishErr = errLedger },
			want: dispatchWant{
				disposition: usecase.R1D4, err: errLedger, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassAccepted},
				steps: []usecase.Step{}, marked: ports.ClassAccepted,
			},
		},
		{
			name: "a failed apply leaves the reception incomplete",
			arrange: func(r *dispatchRig) {
				r.tx.finished = ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassAccepted}
				r.apply.err = usecase.NewFailure(usecase.Conflict, true, errApply)
			},
			want: dispatchWant{
				disposition: usecase.R1D3, err: errApply, category: usecase.Conflict, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassAccepted},
				steps: []usecase.Step{usecase.StepConfirm}, marked: ports.ClassAccepted,
			},
		},
		{
			name: "a failed commit is returned as it is",
			arrange: func(r *dispatchRig) {
				r.tx.finished = ports.Finished{Result: ports.FinishRecorded, Recorded: ports.ClassAccepted}
				r.uow.commitErr = errCommit
			},
			want: dispatchWant{
				disposition: usecase.R1D4, err: errCommit, submits: 1, withins: 1,
				storeFinishes: []ports.ExternalClass{}, txFinishes: []ports.ExternalClass{ports.ClassAccepted},
				steps: []usecase.Step{usecase.StepConfirm}, marked: ports.ClassAccepted, completed: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newDispatchRig()
			tt.arrange(rig)
			disposition, err := usecase.Dispatch(context.Background(), rig.dispatch())

			assertDispatch(t, rig, disposition, err, tt.want)
		})
	}
}

func assertDispatch(t *testing.T, rig *dispatchRig, disposition usecase.Disposition, err error, want dispatchWant) {
	t.Helper()
	if disposition != want.disposition {
		t.Errorf("disposition = %v, want %v", disposition, want.disposition)
	}
	if want.err == nil && want.category == "" && err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if want.err != nil && !errors.Is(err, want.err) {
		t.Errorf("err = %v, want %v", err, want.err)
	}
	if want.category != "" && usecase.CategoryOf(err) != string(want.category) {
		t.Errorf("category of %v = %q, want %q", err, usecase.CategoryOf(err), want.category)
	}
	if len(rig.store.begins) != 1 || rig.store.begins[0] != dispatchMsg || rig.store.nows[0] != rig.clock.at {
		t.Errorf("store begins = %v at %v, want one for %q at %v", rig.store.begins, rig.store.nows, dispatchMsg, rig.clock.at)
	}
	if len(rig.tx.begins) != 0 {
		t.Errorf("transactional begins = %v, want none", rig.tx.begins)
	}
	if rig.command.calls != want.submits || rig.command.inTx {
		t.Errorf("submits = %d (inside a transaction: %v), want %d outside any", rig.command.calls, rig.command.inTx, want.submits)
	}
	if rig.uow.calls != want.withins {
		t.Errorf("withins = %d, want %d", rig.uow.calls, want.withins)
	}
	if got := rig.store.finishedClasses(); !slices.Equal(got, want.storeFinishes) {
		t.Errorf("store finishes = %v, want %v", got, want.storeFinishes)
	}
	if got := rig.tx.finishedClasses(); !slices.Equal(got, want.txFinishes) {
		t.Errorf("transactional finishes = %v, want %v", got, want.txFinishes)
	}
	if got := rig.apply.stepsTaken(); !slices.Equal(got, want.steps) || rig.apply.outTx {
		t.Errorf("applied steps = %v (outside a transaction: %v), want %v", got, rig.apply.outTx, want.steps)
	}
	if class, conflict := rig.inst.marked(); class != want.marked || conflict != want.conflict {
		t.Errorf("marked class %v, conflict %v; want %v, %v", class, conflict, want.marked, want.conflict)
	}
	wantOutcome := ports.OutcomeAccepted
	if err != nil {
		wantOutcome = ports.OutcomeFailed
	}
	if !slices.Equal(rig.inst.operations, []string{"external.dispatch"}) || rig.inst.outcome() != wantOutcome || (err != nil && rig.inst.results[0].Err != err) {
		t.Errorf("instrumented %v ending %+v, want one external.dispatch ending %s", rig.inst.operations, rig.inst.results, wantOutcome)
	}
	for _, o := range rig.tx.finishes {
		if o.Presumed != want.presumed {
			t.Errorf("transactional finish %+v presumed = %v, want %v", o, o.Presumed, want.presumed)
		}
	}
	if rig.pending.Completed() != want.completed {
		t.Errorf("reception completed = %v, want %v", rig.pending.Completed(), want.completed)
	}
}

func TestDispatchHandsTheStepWhatItKnows(t *testing.T) {
	rig := newDispatchRig()
	rig.command.class = ports.ClassUnknownSent
	rig.command.obs = ports.Observation{Key: dispatchKey, Finding: ports.FindingPending, ID: "o-1"}
	rig.store.start = ports.StartedAttempt(openedAttempt(3))

	if _, err := usecase.Dispatch(context.Background(), rig.dispatch()); err != nil {
		t.Fatalf("Dispatch() = %v", err)
	}

	want := usecase.ExternalStep{Step: usecase.StepAwait, Attempt: openedAttempt(3), Class: ports.ClassUnknownSent, Observation: rig.command.obs}
	if len(rig.apply.steps) != 1 || rig.apply.steps[0] != want {
		t.Fatalf("applied = %+v, want %+v", rig.apply.steps, want)
	}
	if len(rig.command.keys) != 1 || rig.command.keys[0] != dispatchKey || string(rig.command.payloads[0]) != "payload" {
		t.Fatalf("submitted %v %q, want the key and payload once", rig.command.keys, rig.command.payloads)
	}
	finish := ports.AttemptOutcome{Key: dispatchKey, N: 3, Class: ports.ClassUnknownSent, Observation: rig.command.obs, FinishedAt: rig.clock.at}
	if len(rig.tx.finishes) != 1 || rig.tx.finishes[0] != finish {
		t.Fatalf("finished %+v, want %+v", rig.tx.finishes, finish)
	}
	completion := ports.Completion{Status: ports.StatusProcessed, At: rig.clock.at}
	if len(rig.pending.completions) != 1 || rig.pending.completions[0].Status != completion.Status || rig.pending.completions[0].At != completion.At {
		t.Fatalf("completions = %+v, want %+v", rig.pending.completions, completion)
	}
	if len(rig.inbox.receipts) != 1 || rig.inbox.receipts[0].MessageID != dispatchMsg {
		t.Fatalf("receipts = %+v, want the dispatch receipt once", rig.inbox.receipts)
	}
}

func TestDispatchRefusesAnIncompleteDispatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(d *usecase.ExternalDispatch[extResources])
		want   error
	}{
		{"no unit of work", func(d *usecase.ExternalDispatch[extResources]) { d.UoW = nil }, usecase.ErrIncompleteDispatch},
		{"no store", func(d *usecase.ExternalDispatch[extResources]) { d.Store = nil }, usecase.ErrIncompleteDispatch},
		{"no ledger accessor", func(d *usecase.ExternalDispatch[extResources]) { d.Ledger = nil }, usecase.ErrIncompleteDispatch},
		{"no inbox accessor", func(d *usecase.ExternalDispatch[extResources]) { d.Inbox = nil }, usecase.ErrIncompleteDispatch},
		{"no command", func(d *usecase.ExternalDispatch[extResources]) { d.Command = nil }, usecase.ErrIncompleteDispatch},
		{"no clock", func(d *usecase.ExternalDispatch[extResources]) { d.Clock = nil }, usecase.ErrIncompleteDispatch},
		{"no apply", func(d *usecase.ExternalDispatch[extResources]) { d.Apply = nil }, usecase.ErrIncompleteDispatch},
		{"no settle context", func(d *usecase.ExternalDispatch[extResources]) { d.SettleContext = nil }, usecase.ErrIncompleteDispatch},
		{"no instrumentation", func(d *usecase.ExternalDispatch[extResources]) { d.Instrumentation = nil }, usecase.ErrIncompleteDispatch},
		{"no key", func(d *usecase.ExternalDispatch[extResources]) { d.Key = "" }, usecase.ErrIncompleteDispatch},
		{"no stale cutoff", func(d *usecase.ExternalDispatch[extResources]) { d.StaleAfter = 0 }, usecase.ErrIncompleteDispatch},
		{"a negative stale cutoff", func(d *usecase.ExternalDispatch[extResources]) { d.StaleAfter = -1 }, usecase.ErrIncompleteDispatch},
		{"an invalid profile", func(d *usecase.ExternalDispatch[extResources]) { d.Profile.MaxQueries = 0 }, usecase.ErrInvalidExternalProfile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newDispatchRig()
			d := rig.dispatch()
			tt.mutate(&d)

			disposition, err := usecase.Dispatch(context.Background(), d)
			if !errors.Is(err, tt.want) || disposition != usecase.R1D4 {
				t.Fatalf("Dispatch() = %v, %v; want R1×D4, %v", disposition, err, tt.want)
			}
			if len(rig.store.begins) != 0 || rig.command.calls != 0 || rig.uow.calls != 0 {
				t.Fatalf("an incomplete dispatch reached the ledger, the partner or the transaction")
			}
		})
	}
}

func TestDispatchRecordsTheOutcomeAfterTheCallerGaveUp(t *testing.T) {
	tests := []struct {
		name          string
		class         ports.ExternalClass
		wantStore     int
		wantWithins   int
		wantCompleted bool
	}{
		{"an unanswered submission still settles in the transaction", ports.ClassUnknownSent, 0, 1, true},
		{"a spent budget still finishes through the store", ports.ClassRetryableTransport, 1, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newDispatchRig()
			ctx, cancel := context.WithCancel(context.Background())
			rig.command.class, rig.command.after = tt.class, cancel

			if _, err := usecase.Dispatch(ctx, rig.dispatch()); err != nil && tt.wantWithins > 0 {
				t.Fatalf("Dispatch() = %v, want the outcome settled", err)
			}
			if len(rig.store.finishes) != tt.wantStore || rig.uow.calls != tt.wantWithins || rig.pending.Completed() != tt.wantCompleted {
				t.Fatalf("store finishes %d, withins %d, completed %v; want %d, %d, %v",
					len(rig.store.finishes), rig.uow.calls, rig.pending.Completed(), tt.wantStore, tt.wantWithins, tt.wantCompleted)
			}
			for _, err := range append(rig.store.finishErrs, rig.tx.finishErrs...) {
				if err != nil {
					t.Fatalf("the outcome was recorded under a cancelled context: %v", err)
				}
			}
		})
	}
}
