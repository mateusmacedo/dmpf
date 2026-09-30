package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	second      = int64(1_000_000_000)
	millisecond = int64(1_000_000)
	now         = ports.Instant(1_000 * second)
)

type fakePending struct{ completions []ports.Completion }

func (p *fakePending) Complete(_ context.Context, c ports.Completion) error {
	p.completions = append(p.completions, c)
	return nil
}

func (p *fakePending) Completed() bool { return len(p.completions) > 0 }

type fakeInbox struct {
	reception ports.Reception
	err       error
	receipts  []ports.Receipt
}

func (f *fakeInbox) Register(_ context.Context, r ports.Receipt) (ports.Reception, error) {
	f.receipts = append(f.receipts, r)
	return f.reception, f.err
}

func firstInbox() (*fakeInbox, *fakePending) {
	pending := &fakePending{}
	return &fakeInbox{reception: ports.FirstReception(pending)}, pending
}

func commandContext(t *testing.T, deadline ports.Instant, key string) context.Context {
	t.Helper()
	tenant := ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1",
		Tenant: &tenant, Deadline: deadline, Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	ctx := ports.WithIdempotencySlot(ports.WithExecutionContext(context.Background(), execution))
	if key != "" {
		ctx = ports.WithIdempotencyKey(ctx, key)
	}
	return ctx
}

func prefixDigest(canonical []byte) ports.Fingerprint {
	var fingerprint ports.Fingerprint
	copy(fingerprint[:], canonical)
	return fingerprint
}

func addItemCommand(inbox ports.Inbox, runs *int) usecase.IdempotentCommand[itemAccepted] {
	return usecase.IdempotentCommand[itemAccepted]{
		Inbox:       inbox,
		Consumer:    "orders.commands",
		Operation:   "orders.AddItem",
		Fingerprint: usecase.NewFingerprint("orders.AddItem").String("o-1"),
		Now:         now,
		Policy:      usecase.IdempotencyPolicy{Wait: second, Retention: 24 * 3600 * second, Digest: prefixDigest},
		Codec:       itemAcceptedCodec,
		Run: func() (usecase.Outcome[itemAccepted], error) {
			*runs++
			return usecase.Accepted(itemAccepted{Order: "o-1", Items: 1}), nil
		},
	}
}

func markOf(ctx context.Context) ports.IdempotencyOutcome {
	outcome, _ := ports.IdempotencyOutcomeFrom(ctx)
	return outcome
}

func far() ports.Instant { return now + ports.Instant(10*second) }

func TestAFirstCommandRunsAndStoresItsOutcome(t *testing.T) {
	inbox, pending := firstInbox()
	ctx := commandContext(t, far(), "k-1")
	runs := 0

	outcome, replayed, err := usecase.RunIdempotent(ctx, addItemCommand(inbox, &runs))
	if err != nil {
		t.Fatalf("RunIdempotent() = %v, want nil", err)
	}
	if replayed || runs != 1 {
		t.Fatalf("replayed = %v, runs = %d; want false, 1", replayed, runs)
	}
	if len(pending.completions) != 1 || pending.completions[0].Status != ports.StatusProcessed {
		t.Fatalf("completions = %+v, want one processed", pending.completions)
	}
	stored, err := usecase.DecodeOutcome(itemAcceptedCodec, pending.completions[0].Outcome)
	if err != nil || stored.Response() != outcome.Response() {
		t.Fatalf("stored outcome = (%+v, %v), want the returned one", stored.Response(), err)
	}
	if mark := markOf(ctx); mark != ports.IdempotencyNew {
		t.Fatalf("mark = %v, want new", mark)
	}
}

func TestAFirstCommandRefusedByTheDomainIsStoredAsRejected(t *testing.T) {
	inbox, pending := firstInbox()
	cmd := addItemCommand(inbox, new(int))
	cmd.Run = func() (usecase.Outcome[itemAccepted], error) {
		return usecase.Rejected[itemAccepted](kernel.Reject("orders/item-limit-exceeded", "the order is full")), nil
	}

	if _, _, err := usecase.RunIdempotent(commandContext(t, far(), "k-1"), cmd); err != nil {
		t.Fatalf("RunIdempotent() = %v, want nil", err)
	}
	if len(pending.completions) != 1 || pending.completions[0].Status != ports.StatusRejected {
		t.Fatalf("completions = %+v, want one rejected: its replay is R3", pending.completions)
	}
}

func TestARepeatedCommandReplaysWithoutRunning(t *testing.T) {
	refusal := kernel.Reject("orders/item-limit-exceeded", "the order is full")
	for name, c := range map[string]struct {
		reception ports.Reception
		rejected  bool
	}{
		"accepted (R2)": {ports.ProcessedReception().WithStored(usecase.EncodeOutcome(itemAcceptedCodec, usecase.Accepted(itemAccepted{Order: "o-1", Items: 1}))), false},
		"rejected (R3)": {ports.RejectedReception().WithStored(usecase.EncodeOutcome(itemAcceptedCodec, usecase.Rejected[itemAccepted](refusal))), true},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := commandContext(t, far(), "k-1")
			runs := 0

			outcome, replayed, err := usecase.RunIdempotent(ctx, addItemCommand(&fakeInbox{reception: c.reception}, &runs))
			if err != nil {
				t.Fatalf("RunIdempotent() = %v, want nil", err)
			}
			if !replayed || runs != 0 {
				t.Fatalf("replayed = %v, runs = %d; want true, 0: a replay never reaches the domain", replayed, runs)
			}
			if _, rejected := outcome.Rejection(); rejected != c.rejected {
				t.Fatalf("rejected = %v, want %v", rejected, c.rejected)
			}
			if mark := markOf(ctx); mark != ports.IdempotencyReplayed {
				t.Fatalf("mark = %v, want replayed", mark)
			}
		})
	}
}

func TestACollidingCommandIsAMismatch(t *testing.T) {
	ctx := commandContext(t, far(), "k-1")
	runs := 0

	_, _, err := usecase.RunIdempotent(ctx, addItemCommand(&fakeInbox{reception: ports.CollisionReception()}, &runs))
	if !errors.Is(err, ports.ErrIdempotencyMismatch) || runs != 0 {
		t.Fatalf("RunIdempotent() = %v with %d runs; want ErrIdempotencyMismatch and no run", err, runs)
	}
	if mark := markOf(ctx); mark != ports.IdempotencyMismatch {
		t.Fatalf("mark = %v, want mismatch", mark)
	}
}

func TestACommandWaitingPastItsCeilingIsInFlight(t *testing.T) {
	ctx := commandContext(t, far(), "k-1")
	runs := 0

	_, _, err := usecase.RunIdempotent(ctx, addItemCommand(&fakeInbox{err: ports.ErrRegisterTimeout}, &runs))
	if !errors.Is(err, ports.ErrIdempotencyInFlight) || runs != 0 {
		t.Fatalf("RunIdempotent() = %v with %d runs; want ErrIdempotencyInFlight and no run", err, runs)
	}
	if mark := markOf(ctx); mark != ports.IdempotencyInFlight {
		t.Fatalf("mark = %v, want in_flight", mark)
	}
}

func TestACommandWithoutAValidKeyIsRefusedBeforeTheInbox(t *testing.T) {
	for name, c := range map[string]struct {
		key  string
		want error
	}{
		"absent":    {"", ports.ErrIdempotencyKeyAbsent},
		"malformed": {"k 1", ports.ErrIdempotencyKeyInvalid},
		"too long":  {strings.Repeat("k", 129), ports.ErrIdempotencyKeyInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			inbox, _ := firstInbox()
			runs := 0

			_, _, err := usecase.RunIdempotent(commandContext(t, far(), c.key), addItemCommand(inbox, &runs))
			if !errors.Is(err, c.want) {
				t.Fatalf("RunIdempotent() = %v, want %v", err, c.want)
			}
			if len(inbox.receipts) != 0 || runs != 0 {
				t.Fatalf("receipts = %d, runs = %d; want none", len(inbox.receipts), runs)
			}
		})
	}
}

func TestACommandItCannotFingerprintIsRefusedBeforeTheInbox(t *testing.T) {
	for name, incomplete := range map[string]func(*usecase.IdempotentCommand[itemAccepted]){
		"no fingerprint": func(c *usecase.IdempotentCommand[itemAccepted]) { c.Fingerprint = nil },
		"no digest":      func(c *usecase.IdempotentCommand[itemAccepted]) { c.Policy.Digest = nil },
	} {
		t.Run(name, func(t *testing.T) {
			inbox, _ := firstInbox()
			runs := 0
			cmd := addItemCommand(inbox, &runs)
			incomplete(&cmd)

			_, _, err := usecase.RunIdempotent(commandContext(t, far(), "k-1"), cmd)
			if !errors.Is(err, usecase.ErrIncompleteCommand) {
				t.Fatalf("RunIdempotent() = %v, want ErrIncompleteCommand", err)
			}
			if len(inbox.receipts) != 0 || runs != 0 {
				t.Fatalf("receipts = %d, runs = %d; want none", len(inbox.receipts), runs)
			}
		})
	}
}

func TestTheWaitCeilingStaysInsideTheDeadline(t *testing.T) {
	for name, c := range map[string]struct {
		deadline  ports.Instant
		waitUntil ports.Instant
	}{
		"far deadline keeps the configured wait": {far(), now + ports.Instant(second)},
		"near deadline leaves a margin":          {now + ports.Instant(300*millisecond), now + ports.Instant(200*millisecond)},
		"deadline inside the margin never waits": {now + ports.Instant(50*millisecond), now},
		"deadline already past never waits":      {now - ports.Instant(second), now},
	} {
		t.Run(name, func(t *testing.T) {
			inbox, _ := firstInbox()
			if _, _, err := usecase.RunIdempotent(commandContext(t, c.deadline, "k-1"), addItemCommand(inbox, new(int))); err != nil {
				t.Fatalf("RunIdempotent() = %v, want nil", err)
			}
			if got := inbox.receipts[0].WaitUntil; got != c.waitUntil {
				t.Fatalf("WaitUntil = %d, want %d", got, c.waitUntil)
			}
		})
	}
}

func TestTheReceiptCarriesTheCommandIdentity(t *testing.T) {
	inbox, _ := firstInbox()
	cmd := addItemCommand(inbox, new(int))

	if _, _, err := usecase.RunIdempotent(commandContext(t, far(), "k-1"), cmd); err != nil {
		t.Fatalf("RunIdempotent() = %v, want nil", err)
	}
	want := ports.Receipt{
		Consumer:    "orders.commands",
		MessageID:   "k-1",
		MessageType: "orders.AddItem",
		PayloadHash: "730e6f72646572732e4164644974656d730e6f72646572732e4164644974656d",
		ReceivedAt:  now,
		WaitUntil:   now + ports.Instant(second),
		ExpiresAt:   now + ports.Instant(24*3600*second),
	}
	if got := inbox.receipts[0]; got != want {
		t.Fatalf("receipt = %+v\nwant      %+v", got, want)
	}
}

func TestTheOperationLeadsTheDigestWhateverTheFingerprint(t *testing.T) {
	hashOf := func(operation string) string {
		inbox, _ := firstInbox()
		cmd := addItemCommand(inbox, new(int))
		cmd.Operation = operation
		cmd.Fingerprint = new(usecase.Fingerprint).String("o-1")
		if _, _, err := usecase.RunIdempotent(commandContext(t, far(), "k-1"), cmd); err != nil {
			t.Fatalf("RunIdempotent(%s) = %v, want nil", operation, err)
		}
		return inbox.receipts[0].PayloadHash
	}

	if hashOf("orders.AddItem") == hashOf("orders.RemoveItem") {
		t.Fatal("two operations with the same fingerprint share a payload hash: the key reused for another operation would replay instead of R4")
	}
}

func TestAFailedRunCompletesNothing(t *testing.T) {
	boom := errors.New("application_test: run failed")
	inbox, pending := firstInbox()
	cmd := addItemCommand(inbox, new(int))
	cmd.Run = func() (usecase.Outcome[itemAccepted], error) { return usecase.Outcome[itemAccepted]{}, boom }

	_, _, err := usecase.RunIdempotent(commandContext(t, far(), "k-1"), cmd)
	if !errors.Is(err, boom) {
		t.Fatalf("RunIdempotent() = %v, want the run's error", err)
	}
	if len(pending.completions) != 0 {
		t.Fatal("Complete called after a failed run; the rollback would carry an entry for an effect that never happened")
	}
}

func TestAnUnreadableStoredOutcomeNeverRunsTheCommandAgain(t *testing.T) {
	runs := 0
	inbox := &fakeInbox{reception: ports.ProcessedReception().WithStored([]byte{0x7f})}

	_, _, err := usecase.RunIdempotent(commandContext(t, far(), "k-1"), addItemCommand(inbox, &runs))
	if !errors.Is(err, usecase.ErrOutcomeUnreadable) || runs != 0 {
		t.Fatalf("RunIdempotent() = %v with %d runs; want ErrOutcomeUnreadable and no run", err, runs)
	}
}
