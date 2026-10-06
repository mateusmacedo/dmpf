package application_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type withinKey struct{}

type execResources struct{ inbox ports.Inbox }

type execTrail struct {
	steps    []string
	result   ports.Result
	audits   []ports.AuditEvent
	auditCtx context.Context
	runCtx   context.Context
	identity usecase.Identity
}

func (t *execTrail) step(format string, args ...any) {
	t.steps = append(t.steps, fmt.Sprintf(format, args...))
}

func (t *execTrail) BeginOperation(ctx context.Context, operation string) (context.Context, ports.EndOperation) {
	t.step("begin %s", operation)
	return context.WithValue(ctx, spanKey{}, operation), func(result ports.Result) {
		t.step("end %s", result.Outcome)
		t.result = result
	}
}

func (t *execTrail) Audit(ctx context.Context, event ports.AuditEvent) {
	t.step("audit")
	t.audits = append(t.audits, event)
	t.auditCtx = ctx
}

func (t *execTrail) Now() ports.Instant {
	t.step("clock.Now")
	return now
}

func (t *execTrail) NewMessageID() ports.MessageID {
	t.step("ids.NewMessageID")
	return ports.MessageID(fmt.Sprintf("m-%d", len(t.steps)))
}

func (t *execTrail) Within(ctx context.Context, fn func(context.Context, execResources) error) error {
	t.step("within")
	return fn(context.WithValue(ctx, withinKey{}, "tx"), execResources{inbox: trailInbox{t, &fakeInbox{}}})
}

type trailInbox struct {
	trail *execTrail
	inbox *fakeInbox
}

func (i trailInbox) Register(ctx context.Context, r ports.Receipt) (ports.Reception, error) {
	i.trail.step("inbox.Register at %d", r.ReceivedAt)
	return i.inbox.Register(ctx, r)
}

type execCase struct {
	trail     *execTrail
	executor  usecase.Executor[execResources, string]
	command   usecase.Command[execResources, string, itemAccepted]
	reception ports.Reception
	inboxErr  error
}

func newExecCase(t *testing.T) *execCase {
	t.Helper()
	trail := &execTrail{}
	c := &execCase{trail: trail}
	pending := &fakePending{}
	c.reception = ports.FirstReception(pending)
	c.executor = usecase.Executor[execResources, string]{
		UoW: trail,
		Inbox: func(res execResources) ports.Inbox {
			inbox := res.inbox.(trailInbox)
			inbox.inbox.reception, inbox.inbox.err = c.reception, c.inboxErr
			return inbox
		},
		Consumer:  "orders.commands",
		Clock:     trail,
		IDs:       trail,
		MaxEvents: 1,
		Policy:    usecase.IdempotencyPolicy{Wait: second, Retention: 24 * 3600 * second, Digest: prefixDigest},
		Authorize: func(_ context.Context, input string) error {
			trail.step("authorize %s", input)
			return nil
		},
		Instrumentation: trail,
	}
	c.command = usecase.Command[execResources, string, itemAccepted]{
		Operation:   "orders.AddItem",
		Object:      "o-1",
		Input:       "o-1",
		Fingerprint: usecase.NewFingerprint("orders.AddItem").String("o-1"),
		Codec:       itemAcceptedCodec,
		Run: func(ctx context.Context, _ execResources, identity usecase.Identity) (usecase.Outcome[itemAccepted], error) {
			trail.step("run")
			trail.runCtx, trail.identity = ctx, identity
			return usecase.Accepted(itemAccepted{Order: "o-1", Items: 1}), nil
		},
	}
	return c
}

func (c *execCase) execute(t *testing.T) (usecase.Outcome[itemAccepted], error) {
	t.Helper()
	return usecase.Execute(commandContext(t, far(), "k-1"), c.executor, c.command)
}

func TestExecuteWalksTheStepsInTheirNormativeOrder(t *testing.T) {
	c := newExecCase(t)

	outcome, err := c.execute(t)

	if err != nil || outcome.Response() != (itemAccepted{Order: "o-1", Items: 1}) {
		t.Fatalf("Execute() = (%+v, %v), want the accepted response", outcome.Response(), err)
	}
	want := []string{
		"begin orders.AddItem", "authorize o-1", "clock.Now", "ids.NewMessageID",
		"within", fmt.Sprintf("inbox.Register at %d", now), "run", "end accepted", "audit",
	}
	if !slices.Equal(c.trail.steps, want) {
		t.Fatalf("steps = %q, want %q", c.trail.steps, want)
	}
}

func TestExecuteStopsAtTheAuthorizer(t *testing.T) {
	denied := fmt.Errorf("%w: the subject does not hold orders:write", ports.ErrDenied)
	broken := errors.New("authorizer unavailable")
	for name, tc := range map[string]struct {
		err  error
		want ports.Result
	}{
		"denied": {denied, ports.Result{Outcome: ports.OutcomeDenied}},
		"failed": {broken, ports.Result{Outcome: ports.OutcomeFailed, Err: broken}},
	} {
		t.Run(name, func(t *testing.T) {
			c := newExecCase(t)
			c.executor.Authorize = func(context.Context, string) error { return tc.err }

			_, err := c.execute(t)

			if err != tc.err {
				t.Fatalf("err = %v, want the authorizer's error unchanged", err)
			}
			if c.trail.result != tc.want {
				t.Fatalf("end(%+v), want end(%+v)", c.trail.result, tc.want)
			}
			if want := []string{"begin orders.AddItem", "end " + string(tc.want.Outcome)}; !slices.Equal(c.trail.steps, want) {
				t.Fatalf("steps = %q, want %q", c.trail.steps, want)
			}
		})
	}
}

func TestExecuteReplaysWithoutRunningOrAuditing(t *testing.T) {
	refusal := kernel.Reject("orders/item-limit-exceeded", "the order is full")
	for name, tc := range map[string]struct {
		reception ports.Reception
		category  ports.OutcomeCategory
	}{
		"accepted (R2)": {ports.ProcessedReception().WithStored(usecase.EncodeOutcome(itemAcceptedCodec, usecase.Accepted(itemAccepted{Order: "o-1", Items: 3}))), ports.OutcomeAccepted},
		"rejected (R3)": {ports.RejectedReception().WithStored(usecase.EncodeOutcome(itemAcceptedCodec, usecase.Rejected[itemAccepted](refusal))), ports.OutcomeRejected},
	} {
		t.Run(name, func(t *testing.T) {
			c := newExecCase(t)
			c.reception = tc.reception

			outcome, err := c.execute(t)

			if err != nil {
				t.Fatalf("Execute() = %v, want nil", err)
			}
			if outcome.Category() != tc.category {
				t.Fatalf("category = %s, want %s", outcome.Category(), tc.category)
			}
			if slices.Contains(c.trail.steps, "run") || slices.Contains(c.trail.steps, "audit") {
				t.Fatalf("steps = %q, want neither run nor audit on a replay", c.trail.steps)
			}
			if c.trail.result != (ports.Result{Outcome: tc.category}) {
				t.Fatalf("end(%+v), want end(%s)", c.trail.result, tc.category)
			}
		})
	}
}

func TestExecuteAuditsARejectedOutcome(t *testing.T) {
	c := newExecCase(t)
	c.command.Run = func(context.Context, execResources, usecase.Identity) (usecase.Outcome[itemAccepted], error) {
		return usecase.Rejected[itemAccepted](kernel.Reject("orders/item-limit-exceeded", "the order is full")), nil
	}

	outcome, err := c.execute(t)

	if err != nil || outcome.Category() != ports.OutcomeRejected {
		t.Fatalf("Execute() = (%s, %v), want (rejected, nil)", outcome.Category(), err)
	}
	if c.trail.result != (ports.Result{Outcome: ports.OutcomeRejected}) {
		t.Fatalf("end(%+v), want end(rejected)", c.trail.result)
	}
	if len(c.trail.audits) != 1 || c.trail.audits[0].Outcome != ports.OutcomeRejected {
		t.Fatalf("audits = %+v, want one rejected", c.trail.audits)
	}
}

func TestExecuteReturnsTheFailureUnwrappedWithoutAuditing(t *testing.T) {
	boom := errors.New("boom")
	collision := ports.CollisionReception()
	for name, tc := range map[string]struct {
		arrange func(*execCase)
		want    error
		message string
	}{
		"run fails": {
			arrange: func(c *execCase) {
				c.command.Run = func(context.Context, execResources, usecase.Identity) (usecase.Outcome[itemAccepted], error) {
					return usecase.Outcome[itemAccepted]{}, boom
				}
			},
			want: boom, message: "boom",
		},
		"mismatch": {
			arrange: func(c *execCase) { c.reception = collision },
			want:    ports.ErrIdempotencyMismatch, message: ports.ErrIdempotencyMismatch.Error(),
		},
		"in flight": {
			arrange: func(c *execCase) { c.inboxErr = ports.ErrRegisterTimeout },
			want:    ports.ErrIdempotencyInFlight,
			message: fmt.Sprintf("%v: %v", ports.ErrIdempotencyInFlight, ports.ErrRegisterTimeout),
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := newExecCase(t)
			tc.arrange(c)

			_, err := c.execute(t)

			if !errors.Is(err, tc.want) || err.Error() != tc.message {
				t.Fatalf("err = %v, want %q matching %v", err, tc.message, tc.want)
			}
			if c.trail.result != (ports.Result{Outcome: ports.OutcomeFailed, Err: err}) {
				t.Fatalf("end(%+v), want end(failed, %v)", c.trail.result, err)
			}
			if len(c.trail.audits) != 0 {
				t.Fatalf("audits = %+v, want none", c.trail.audits)
			}
		})
	}
}

func TestExecuteRefusesACommandWithoutAKey(t *testing.T) {
	c := newExecCase(t)

	_, err := usecase.Execute(commandContext(t, far(), ""), c.executor, c.command)

	if !errors.Is(err, ports.ErrIdempotencyKeyAbsent) || err.Error() != ports.ErrIdempotencyKeyAbsent.Error() {
		t.Fatalf("err = %v, want ErrIdempotencyKeyAbsent unwrapped", err)
	}
	if slices.Contains(c.trail.steps, "run") || len(c.trail.audits) != 0 {
		t.Fatalf("steps = %q, want neither run nor audit", c.trail.steps)
	}
}

func TestExecuteTreatsNilInstrumentationAsInert(t *testing.T) {
	c := newExecCase(t)
	c.executor.Instrumentation = nil

	outcome, err := c.execute(t)

	if err != nil || outcome.Category() != ports.OutcomeAccepted {
		t.Fatalf("Execute() = (%s, %v), want (accepted, nil)", outcome.Category(), err)
	}
}

func TestExecuteResolvesOneIdentityBeforeTheTransaction(t *testing.T) {
	c := newExecCase(t)
	c.executor.MaxEvents = 2

	_, _ = c.execute(t)

	within := slices.Index(c.trail.steps, "within")
	if ids := slices.Index(c.trail.steps[within:], "ids.NewMessageID"); ids != -1 {
		t.Fatalf("steps = %q, want every identifier minted before within", c.trail.steps)
	}
	if c.trail.identity.OccurredAt != now || len(c.trail.identity.MessageIDs) != 2 {
		t.Fatalf("identity = %+v, want two identifiers at %d", c.trail.identity, now)
	}
	want := ports.AuditEvent{Object: "o-1", Action: "orders.AddItem", Outcome: ports.OutcomeAccepted, At: now}
	if len(c.trail.audits) != 1 || c.trail.audits[0] != want {
		t.Fatalf("audits = %+v, want [%+v]", c.trail.audits, want)
	}
}

func TestExecuteRunsInsideTheTransactionAndAuditsInsideTheOperation(t *testing.T) {
	c := newExecCase(t)

	_, _ = c.execute(t)

	if c.trail.runCtx.Value(withinKey{}) != "tx" {
		t.Fatal("Run did not receive the context of the Within callback")
	}
	if c.trail.auditCtx.Value(spanKey{}) != "orders.AddItem" || c.trail.auditCtx.Value(withinKey{}) != nil {
		t.Fatal("Audit did not receive the context of BeginOperation")
	}
}
