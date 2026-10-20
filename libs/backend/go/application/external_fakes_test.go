package application_test

import (
	"context"
	"maps"
	"slices"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type extResources struct {
	ledger ports.AttemptLedger
	inbox  ports.Inbox
}

type extUoW struct {
	res       extResources
	commitErr error
	calls     int
	open      bool
}

func (u *extUoW) Within(ctx context.Context, fn func(context.Context, extResources) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	u.calls++
	u.open = true
	defer func() { u.open = false }()
	if err := fn(ctx, u.res); err != nil {
		return err
	}
	return u.commitErr
}

type extLedger struct {
	start      ports.AttemptStart
	beginErr   error
	finished   ports.Finished
	finishErr  error
	observed   ports.Observed
	observeErr error

	begins     []ports.MessageID
	finishes   []ports.AttemptOutcome
	finishErrs []error
	observes   []ports.Observation
	nows       []ports.Instant
}

func (l *extLedger) Begin(_ context.Context, _ ports.OutboundKey, message ports.MessageID, now ports.Instant) (ports.AttemptStart, error) {
	l.begins = append(l.begins, message)
	l.nows = append(l.nows, now)
	return l.start, l.beginErr
}

func (l *extLedger) Finish(ctx context.Context, o ports.AttemptOutcome) (ports.Finished, error) {
	l.finishes = append(l.finishes, o)
	l.finishErrs = append(l.finishErrs, ctx.Err())
	return l.finished, l.finishErr
}

func (l *extLedger) Observe(_ context.Context, o ports.Observation, now ports.Instant) (ports.Observed, error) {
	l.observes = append(l.observes, o)
	l.nows = append(l.nows, now)
	return l.observed, l.observeErr
}

func (l *extLedger) Stale(context.Context, ports.Instant, ports.AttemptCursor, int) ([]ports.Attempt, error) {
	return nil, nil
}

func (l *extLedger) finishedClasses() []ports.ExternalClass {
	classes := []ports.ExternalClass{}
	for _, o := range l.finishes {
		classes = append(classes, o.Class)
	}
	return classes
}

type extCommand struct {
	class ports.ExternalClass
	obs   ports.Observation
	err   error
	after func()

	uow      *extUoW
	calls    int
	inTx     bool
	keys     []ports.OutboundKey
	payloads [][]byte
}

func (c *extCommand) Submit(_ context.Context, key ports.OutboundKey, payload []byte) (ports.ExternalClass, ports.Observation, error) {
	c.calls++
	c.inTx = c.inTx || c.uow.open
	c.keys = append(c.keys, key)
	c.payloads = append(c.payloads, payload)
	if c.after != nil {
		c.after()
	}
	return c.class, c.obs, c.err
}

type extApply struct {
	err   error
	uow   *extUoW
	steps []usecase.ExternalStep
	outTx bool
}

func (a *extApply) apply(_ context.Context, _ extResources, step usecase.ExternalStep) error {
	a.steps = append(a.steps, step)
	a.outTx = a.outTx || !a.uow.open
	return a.err
}

func (a *extApply) stepsTaken() []usecase.Step {
	steps := []usecase.Step{}
	for _, s := range a.steps {
		steps = append(steps, s.Step)
	}
	return steps
}

type extInstrumentation struct {
	operations []string
	results    []ports.Result
	classes    []ports.ExternalClass
	conflicts  []bool
}

func (i *extInstrumentation) BeginOperation(ctx context.Context, operation string) (context.Context, ports.EndOperation) {
	i.operations = append(i.operations, operation)
	ctx = ports.WithExternalConflictSlot(ports.WithExternalClassSlot(ctx))
	return ctx, func(result ports.Result) {
		class, _ := ports.ExternalClassFrom(ctx)
		i.results = append(i.results, result)
		i.classes = append(i.classes, class)
		i.conflicts = append(i.conflicts, ports.ExternalConflictFrom(ctx))
	}
}

func (*extInstrumentation) Audit(context.Context, ports.AuditEvent) {}

func (i *extInstrumentation) marked() (ports.ExternalClass, bool) {
	if len(i.classes) != 1 {
		return 0, false
	}
	return i.classes[0], i.conflicts[0]
}

func (i *extInstrumentation) outcome() ports.OutcomeCategory {
	if len(i.results) != 1 {
		return ""
	}
	return i.results[0].Outcome
}

func settleWithin(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(ctx)
}

// txLedger keeps the attempt state a rollback must restore, which extLedger's
// canned results cannot show.
type txLedger struct {
	finished map[int]ports.ExternalClass
	observed []string
}

func newTxLedger() *txLedger { return &txLedger{finished: map[int]ports.ExternalClass{}} }

func (l *txLedger) Begin(context.Context, ports.OutboundKey, ports.MessageID, ports.Instant) (ports.AttemptStart, error) {
	return ports.AttemptStart{}, errLedger
}

func (l *txLedger) Finish(_ context.Context, o ports.AttemptOutcome) (ports.Finished, error) {
	switch class, ok := l.finished[o.N]; {
	case !ok:
		l.finished[o.N] = o.Class
		return ports.Finished{Result: ports.FinishRecorded, Recorded: o.Class}, nil
	case class == o.Class || o.Presumed:
		return ports.Finished{Result: ports.FinishUnchanged, Recorded: class}, nil
	default:
		return ports.Finished{Result: ports.FinishConflict, Recorded: class}, nil
	}
}

func (l *txLedger) Observe(_ context.Context, o ports.Observation, _ ports.Instant) (ports.Observed, error) {
	l.observed = append(l.observed, o.ID)
	return ports.Observed{Result: ports.ObserveRecorded, N: 1, Class: l.finished[1], QueriesDone: len(l.observed)}, nil
}

func (l *txLedger) Stale(context.Context, ports.Instant, ports.AttemptCursor, int) ([]ports.Attempt, error) {
	return nil, nil
}

type txLedgerUoW struct {
	ledger    *txLedger
	commitErr error
}

func (u *txLedgerUoW) Within(ctx context.Context, fn func(context.Context, extResources) error) error {
	finished := maps.Clone(u.ledger.finished)
	observed := slices.Clone(u.ledger.observed)
	err := fn(ctx, extResources{ledger: u.ledger})
	if err == nil {
		err = u.commitErr
	}
	if err != nil {
		u.ledger.finished, u.ledger.observed = finished, observed
	}
	return err
}
