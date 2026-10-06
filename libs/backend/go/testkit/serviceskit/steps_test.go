package serviceskit_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
)

type kitResources struct {
	Counters ports.Repository[counterID, counterState]
	Outbox   ports.Outbox
	Commands ports.Inbox
}

func kitBind(tx serviceskit.Tx) kitResources {
	return kitResources{
		Counters: countersTable.Repository(tx.Memory()),
		Outbox:   tx.Outbox(),
		Commands: tx.CommandInbox("counters.commands"),
	}
}

type kitEvent struct{}

func (kitEvent) EventName() string { return "counted" }

func TestStepsRecordInOrderAndResetForgetsThem(t *testing.T) {
	steps := &serviceskit.Steps{}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { steps.Record("concurrent") })
	}
	wg.Wait()

	observed := steps.Observed()
	if len(observed) != 8 {
		t.Fatalf("Observed() = %v, want 8 steps", observed)
	}
	observed[0] = "mutated"
	if steps.Observed()[0] != "concurrent" {
		t.Fatal("Observed() handed out its own slice")
	}
	steps.Reset()
	if got := steps.Observed(); len(got) != 0 {
		t.Fatalf("Observed() after Reset = %v, want none", got)
	}
}

func TestUnitOfWorkRecordsTheTransactionAndCountsWhatTheServiceOpened(t *testing.T) {
	f := serviceskit.NewFakes()
	uow := serviceskit.UnitOfWork(f, kitBind)
	ctx := withExecution(t, context.Background())

	seed := func(ctx context.Context, res kitResources) error {
		return res.Counters.Save(ctx, "c-1", counterState{ID: "c-1", Total: 1, Limit: 2}, 0)
	}
	if err := uow.Within(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	f.MarkSeeded()
	f.Steps.Reset()

	boom := errors.New("boom")
	if err := uow.Within(ctx, func(context.Context, kitResources) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("Within() = %v, want boom", err)
	}
	if err := uow.Within(ctx, func(context.Context, kitResources) error { return nil }); err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	if want := []string{"within", "within", "commit"}; !slices.Equal(f.Steps.Observed(), want) {
		t.Fatalf("steps = %v, want %v: commit only after a transaction that returned nil", f.Steps.Observed(), want)
	}
	if f.WithinCalls() != 2 || f.Commits() != 1 || f.Binds() != 3 {
		t.Fatalf("within %d, commits %d, binds %d; want 2, 1 and 3", f.WithinCalls(), f.Commits(), f.Binds())
	}
	if got := f.Ledger.Count(serviceskit.Rollback); got != 1 {
		t.Fatalf("Ledger.Count(rollback) = %d, want 1", got)
	}
}

func TestTheRecordingPortsWriteTheirSteps(t *testing.T) {
	f := serviceskit.NewFakes()
	uow := serviceskit.UnitOfWork(f, kitBind)
	ctx := ports.WithIdempotencySlot(withExecution(t, context.Background()))

	err := uow.Within(ctx, func(ctx context.Context, res kitResources) error {
		reception, err := res.Commands.Register(ctx, ports.Receipt{Consumer: "counters.commands", MessageID: "k-1", MessageType: "counters.Count", PayloadHash: "h", ReceivedAt: 1, ExpiresAt: 2})
		if err != nil {
			return err
		}
		return reception.Match(
			func(p ports.Pending) error {
				if err := res.Outbox.Enqueue(ctx, ports.OutboxEntry{MessageID: "m-1", Event: kitEvent{}}); err != nil {
					return err
				}
				return p.Complete(ctx, ports.Completion{Status: ports.StatusProcessed, At: 1})
			},
			func() error { return errors.New("processed") },
			func() error { return errors.New("rejected") },
			func() error { return errors.New("collision") },
		)
	})
	if err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	want := []string{"within", "commands.Register", "outbox.Enqueue", "commands.Complete", "commit"}
	if !slices.Equal(f.Steps.Observed(), want) {
		t.Fatalf("steps = %v, want %v", f.Steps.Observed(), want)
	}
	if f.Ledger.Count(serviceskit.Enqueue) != 1 || f.Ledger.Count(serviceskit.Register) != 1 {
		t.Fatalf("ledger = %v, want one enqueue and one registration", f.Ledger)
	}
}

func TestFailRegisterRefusesTheCommandAfterRecordingIt(t *testing.T) {
	f := serviceskit.NewFakes()
	f.FailRegister = ports.ErrRegisterTimeout
	uow := serviceskit.UnitOfWork(f, kitBind)

	err := uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res kitResources) error {
		_, err := res.Commands.Register(ctx, ports.Receipt{Consumer: "counters.commands", MessageID: "k-1"})
		return err
	})

	if !errors.Is(err, ports.ErrRegisterTimeout) {
		t.Fatalf("Within() = %v, want ErrRegisterTimeout", err)
	}
	if want := []string{"within", "commands.Register"}; !slices.Equal(f.Steps.Observed(), want) {
		t.Fatalf("steps = %v, want %v", f.Steps.Observed(), want)
	}
}

func TestClockIDsAndAuthorizeRecordTheirSteps(t *testing.T) {
	f := serviceskit.NewFakes()
	denied := errors.New("denied")
	authorize := serviceskit.Authorize(f, func(_ context.Context, cmd string) error {
		if cmd == "deny" {
			return denied
		}
		return nil
	})

	at := f.Clock(memory.FixedClock{At: 42}).Now()
	id := f.IDs(&memory.SequenceIDs{Prefix: "m-"}).NewMessageID()
	allowed := authorize(context.Background(), "allow")
	refused := authorize(context.Background(), "deny")

	if at != 42 || id == "" || allowed != nil || !errors.Is(refused, denied) {
		t.Fatalf("Now = %d, id = %q, allow = %v, deny = %v", at, id, allowed, refused)
	}
	if want := []string{"clock.Now", "ids.NewMessageID", "authorize", "authorize"}; !slices.Equal(f.Steps.Observed(), want) {
		t.Fatalf("steps = %v, want %v", f.Steps.Observed(), want)
	}
}

func TestFoldDigestIsDeterministicAndSensitiveToEveryByte(t *testing.T) {
	a := serviceskit.FoldDigest([]byte("orders.AddItem|P-100"))
	if a != serviceskit.FoldDigest([]byte("orders.AddItem|P-100")) {
		t.Fatal("FoldDigest differs for the same input")
	}
	if a == serviceskit.FoldDigest([]byte("orders.AddItem|P-101")) {
		t.Fatal("FoldDigest ignores the last byte")
	}
}

func TestFaultyPortsFailWhereTheTestAsks(t *testing.T) {
	boom := errors.New("boom")
	f := serviceskit.NewFakes()
	uow := serviceskit.UnitOfWork(f, kitBind)
	ctx := withExecution(t, context.Background())

	for name, tc := range map[string]struct {
		faults serviceskit.Faults
		run    func(context.Context, ports.Repository[counterID, counterState], ports.Outbox) error
	}{
		"load": {serviceskit.Faults{Load: boom}, func(ctx context.Context, r ports.Repository[counterID, counterState], _ ports.Outbox) error {
			_, _, err := r.Load(ctx, "c-1")
			return err
		}},
		"save": {serviceskit.Faults{Save: boom}, func(ctx context.Context, r ports.Repository[counterID, counterState], _ ports.Outbox) error {
			return r.Save(ctx, "c-1", counterState{ID: "c-1", Total: 1, Limit: 2}, 0)
		}},
		"enqueue": {serviceskit.Faults{Enqueue: boom}, func(ctx context.Context, _ ports.Repository[counterID, counterState], o ports.Outbox) error {
			return o.Enqueue(ctx, ports.OutboxEntry{MessageID: "m-1", Event: kitEvent{}})
		}},
	} {
		t.Run(name, func(t *testing.T) {
			err := uow.Within(ctx, func(ctx context.Context, res kitResources) error {
				return tc.run(ctx, serviceskit.FaultyRepository(res.Counters, tc.faults), serviceskit.FaultyOutbox(res.Outbox, tc.faults))
			})
			if !errors.Is(err, boom) {
				t.Fatalf("Within() = %v, want boom", err)
			}
		})
	}

	err := uow.Within(ctx, func(ctx context.Context, res kitResources) error {
		repo := serviceskit.FaultyRepository(res.Counters, serviceskit.Faults{})
		if err := repo.Save(ctx, "c-2", counterState{ID: "c-2", Total: 1, Limit: 2}, 0); err != nil {
			return err
		}
		_, version, err := repo.Load(ctx, "c-2")
		if version != 1 {
			return errors.New("the write did not reach the store")
		}
		return err
	})
	if err != nil {
		t.Fatalf("Within() without faults = %v, want nil", err)
	}
}
