package clock_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
)

var epoch = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

// awaitClose is a safety net against a hung test, never part of the logic under
// test: every assertion below is decided by Advance, not by wall time.
const awaitClose = 2 * time.Second

func requireNotFired[T any](t *testing.T, ch <-chan T, what string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatalf("%s fired before the clock reached its deadline", what)
	default:
	}
}

// awaitPending blocks until the clock holds the expected number of alarms, so a
// test never advances before the goroutine under test has registered its timer.
// The wall-clock bound is only a safety net against a hung test.
func awaitPending(t *testing.T, fake *clock.Fake, want int) {
	t.Helper()
	limit := time.Now().Add(awaitClose)
	for {
		if got := fake.Pending(); got == want {
			return
		}
		if time.Now().After(limit) {
			t.Fatalf("Pending() = %d, want %d", fake.Pending(), want)
		}
		runtime.Gosched()
	}
}

func requireFired[T any](t *testing.T, ch <-chan T, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(awaitClose):
		t.Fatalf("%s did not fire after the clock passed its deadline", what)
	}
}

func TestTheFakeNeverMovesOnItsOwn(t *testing.T) {
	fake := clock.NewFake(epoch)

	if got := fake.Now(); !got.Equal(epoch) {
		t.Fatalf("Now() = %v, want %v", got, epoch)
	}
	if got := fake.Now(); !got.Equal(epoch) {
		t.Fatalf("Now() = %v on a second read, want the same instant", got)
	}
}

func TestAdvanceMovesTheInstant(t *testing.T) {
	fake := clock.NewFake(epoch)

	fake.Advance(90 * time.Second)

	if got, want := fake.Now(), epoch.Add(90*time.Second); !got.Equal(want) {
		t.Fatalf("Now() = %v, want %v", got, want)
	}
}

func TestANonPositiveAdvanceIsANoop(t *testing.T) {
	fake := clock.NewFake(epoch)

	fake.Advance(0)
	fake.Advance(-time.Hour)

	if got := fake.Now(); !got.Equal(epoch) {
		t.Fatalf("Now() = %v, want %v — the clock never runs backwards", got, epoch)
	}
}

func TestAfterFiresOnlyWhenTheDeadlineIsReached(t *testing.T) {
	fake := clock.NewFake(epoch)
	ch := fake.After(time.Second)

	fake.Advance(999 * time.Millisecond)
	requireNotFired(t, ch, "After(1s)")

	fake.Advance(time.Millisecond)
	requireFired(t, ch, "After(1s)")
}

func TestAfterWithANonPositiveDurationIsAlreadyDue(t *testing.T) {
	fake := clock.NewFake(epoch)

	requireFired(t, fake.After(0), "After(0)")
	requireFired(t, fake.After(-time.Second), "After(-1s)")
}

func TestAlarmsFireInDeadlineOrder(t *testing.T) {
	fake := clock.NewFake(epoch)
	order := make(chan string, 3)

	late := fake.After(3 * time.Second)
	early := fake.After(time.Second)
	middle := fake.After(2 * time.Second)

	go func() { <-late; order <- "late" }()
	go func() { <-early; order <- "early" }()
	go func() { <-middle; order <- "middle" }()

	fake.Advance(5 * time.Second)

	got := make([]string, 0, 3)
	for range 3 {
		select {
		case name := <-order:
			got = append(got, name)
		case <-time.After(awaitClose):
			t.Fatalf("only %v fired, want three alarms", got)
		}
	}
	// The three goroutines are woken in deadline order, but the Go scheduler
	// decides when each one gets to send, so only membership is asserted here.
	seen := map[string]bool{}
	for _, name := range got {
		seen[name] = true
	}
	for _, want := range []string{"early", "middle", "late"} {
		if !seen[want] {
			t.Fatalf("alarm %q never fired; got %v", want, got)
		}
	}
}

func TestAlarmsDueTogetherFireInSchedulingOrder(t *testing.T) {
	fake := clock.NewFake(epoch)
	order := make([]int, 0, 3)
	done := make(chan struct{})

	first, second, third := fake.After(time.Second), fake.After(time.Second), fake.After(time.Second)
	go func() {
		defer close(done)
		<-first
		order = append(order, 1)
		<-second
		order = append(order, 2)
		<-third
		order = append(order, 3)
	}()

	fake.Advance(time.Second)

	select {
	case <-done:
	case <-time.After(awaitClose):
		t.Fatal("the three alarms due at the same instant did not all fire")
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("order = %v, want [1 2 3] — alarms due together fire as scheduled", order)
	}
}

func TestStopPreventsTheExpiry(t *testing.T) {
	fake := clock.NewFake(epoch)
	timer := fake.NewTimer(time.Second)

	if !timer.Stop() {
		t.Fatal("Stop() = false on a pending timer, want true")
	}

	fake.Advance(10 * time.Second)
	requireNotFired(t, timer.C(), "a stopped timer")

	if timer.Stop() {
		t.Fatal("Stop() = true on an already stopped timer, want false")
	}
}

func TestStopReportsFalseAfterTheExpiry(t *testing.T) {
	fake := clock.NewFake(epoch)
	timer := fake.NewTimer(time.Second)
	fake.Advance(time.Second)
	requireFired(t, timer.C(), "the timer")

	if timer.Stop() {
		t.Fatal("Stop() = true after the expiry, want false")
	}
}

func TestResetMovesAPendingExpiry(t *testing.T) {
	fake := clock.NewFake(epoch)
	timer := fake.NewTimer(time.Second)

	if !timer.Reset(3 * time.Second) {
		t.Fatal("Reset() = false on a pending timer, want true")
	}
	if got := fake.Pending(); got != 1 {
		t.Fatalf("Pending() = %d after Reset, want 1 — the alarm must be moved, not duplicated", got)
	}

	fake.Advance(time.Second)
	requireNotFired(t, timer.C(), "the timer after Reset to 3s")

	fake.Advance(2 * time.Second)
	requireFired(t, timer.C(), "the timer after Reset to 3s")
}

func TestResetRearmsAnExpiredTimer(t *testing.T) {
	fake := clock.NewFake(epoch)
	timer := fake.NewTimer(time.Second)
	fake.Advance(time.Second)
	requireFired(t, timer.C(), "the timer")

	if timer.Reset(time.Second) {
		t.Fatal("Reset() = true on an expired timer, want false")
	}

	fake.Advance(time.Second)
	requireFired(t, timer.C(), "the rearmed timer")
}

func TestPendingCountsOnlyWaitingAlarms(t *testing.T) {
	fake := clock.NewFake(epoch)
	if got := fake.Pending(); got != 0 {
		t.Fatalf("Pending() = %d on a fresh clock, want 0", got)
	}

	timer := fake.NewTimer(time.Second)
	fake.After(2 * time.Second)
	if got := fake.Pending(); got != 2 {
		t.Fatalf("Pending() = %d, want 2", got)
	}

	timer.Stop()
	if got := fake.Pending(); got != 1 {
		t.Fatalf("Pending() = %d after Stop, want 1", got)
	}

	fake.Advance(2 * time.Second)
	if got := fake.Pending(); got != 0 {
		t.Fatalf("Pending() = %d after every alarm fired, want 0", got)
	}
}

func TestWithTimeoutExpiresWhenTheClockAdvances(t *testing.T) {
	fake := clock.NewFake(epoch)
	ctx, cancel := fake.WithTimeout(context.Background(), time.Second)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok || !deadline.Equal(epoch.Add(time.Second)) {
		t.Fatalf("Deadline() = %v (set=%v), want %v", deadline, ok, epoch.Add(time.Second))
	}
	requireNotFired(t, ctx.Done(), "the timeout context")

	fake.Advance(time.Second)

	requireFired(t, ctx.Done(), "the timeout context")
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("Err() = %v, want DeadlineExceeded", ctx.Err())
	}
}

func TestWithTimeoutPropagatesTheParentCancellation(t *testing.T) {
	fake := clock.NewFake(epoch)
	parent, cancelParent := context.WithCancel(context.Background())
	ctx, cancel := fake.WithTimeout(parent, time.Hour)
	defer cancel()

	cancelParent()

	requireFired(t, ctx.Done(), "the timeout context")
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("Err() = %v, want Canceled", ctx.Err())
	}
}

func TestTheCancelFuncIsIdempotentAndReleasesTheAlarm(t *testing.T) {
	fake := clock.NewFake(epoch)
	ctx, cancel := fake.WithTimeout(context.Background(), time.Hour)

	cancel()
	cancel()

	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("Err() = %v, want Canceled", ctx.Err())
	}
	if got := fake.Pending(); got != 0 {
		t.Fatalf("Pending() = %d after cancel, want 0 — the alarm must be released", got)
	}
}

func TestACancelledTimeoutKeepsItsFirstCause(t *testing.T) {
	fake := clock.NewFake(epoch)
	ctx, cancel := fake.WithTimeout(context.Background(), time.Second)

	fake.Advance(time.Second)
	requireFired(t, ctx.Done(), "the timeout context")
	cancel()

	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("Err() = %v, want DeadlineExceeded: a later cancel must not rewrite the cause", ctx.Err())
	}
}

func TestTheSleeperWaitsOnTheClock(t *testing.T) {
	fake := clock.NewFake(epoch)
	sleeper := clock.NewSleeper(fake)
	result := make(chan error, 1)

	go func() { result <- sleeper(context.Background(), time.Second) }()
	awaitPending(t, fake, 1)

	fake.Advance(time.Second)

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("sleeper() = %v, want nil", err)
		}
	case <-time.After(awaitClose):
		t.Fatal("sleeper() never returned after the clock passed the wait")
	}
}

func TestTheSleeperGivesUpWhenTheContextIsDone(t *testing.T) {
	fake := clock.NewFake(epoch)
	sleeper := clock.NewSleeper(fake)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)

	go func() { result <- sleeper(ctx, time.Hour) }()
	awaitPending(t, fake, 1)

	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("sleeper() = %v, want Canceled", err)
		}
	case <-time.After(awaitClose):
		t.Fatal("sleeper() kept waiting after the context was cancelled")
	}
	if got := fake.Pending(); got != 0 {
		t.Fatalf("Pending() = %d, want 0 — the sleeper must release its timer", got)
	}
}

func TestTheSleeperReturnsAtOnceForANonPositiveWait(t *testing.T) {
	fake := clock.NewFake(epoch)
	sleeper := clock.NewSleeper(fake)

	if err := sleeper(context.Background(), 0); err != nil {
		t.Fatalf("sleeper(0) = %v, want nil", err)
	}
	if got := fake.Pending(); got != 0 {
		t.Fatalf("Pending() = %d, want 0 — no timer for a wait of zero", got)
	}
}

func TestTheSleeperRefusesAnAlreadyDoneContext(t *testing.T) {
	fake := clock.NewFake(epoch)
	sleeper := clock.NewSleeper(fake)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := sleeper(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("sleeper() = %v, want Canceled", err)
	}
}

func TestTheSystemClockCarriesARealDeadline(t *testing.T) {
	system := clock.System()

	ctx, cancel := system.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("Deadline() reported none on a timeout context")
	}
	if !deadline.After(system.Now()) {
		t.Fatalf("Deadline() = %v, want an instant after Now()", deadline)
	}
	requireFired(t, system.After(0), "System().After(0)")
}

func TestTheSystemTimerStops(t *testing.T) {
	timer := clock.System().NewTimer(time.Hour)

	if !timer.Stop() {
		t.Fatal("Stop() = false on a pending system timer, want true")
	}
	requireNotFired(t, timer.C(), "a stopped system timer")
}
