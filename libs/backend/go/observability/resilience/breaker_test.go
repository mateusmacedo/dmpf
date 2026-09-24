package resilience_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
)

var errDependency = errors.New("payments: 503 service unavailable")

func breakerPolicy() resilience.BreakerPolicy {
	sheet := resilience.Defaults("payments")
	policy, ok := sheet.Breaker.Get()
	if !ok {
		panic("the default sheet declares no breaker")
	}
	return policy
}

// guarded wires a breaker over a call whose outcome the test controls.
func guarded(t *testing.T, instruments *metrics.Instruments) (*resilience.Breaker, *clock.Fake, func(err error) error) {
	t.Helper()

	fake := clock.NewFake(start)
	breaker := resilience.NewBreaker("payments", breakerPolicy(), fake, instruments)

	var outcome error
	decorated := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return outcome
	})

	return breaker, fake, func(err error) error {
		outcome = err
		return decorated(context.Background(), remoteOp(time.Second), nil)
	}
}

func TestABurstBelowTheSampleFloorKeepsTheBreakerClosed(t *testing.T) {
	breaker, _, call := guarded(t, nil)

	_ = call(errDependency)
	_ = call(nil)
	_ = call(nil)

	if got := breaker.State(); got != resilience.BreakerClosed {
		t.Fatalf("State() = %v after 3 calls with 1 failure, want closed — 33 %% on three samples is not evidence (RES-10)", got)
	}
}

func TestTheFloorHoldsEvenWithEveryCallFailing(t *testing.T) {
	breaker, _, call := guarded(t, nil)
	floor := breakerPolicy().MinSamples

	for range floor - 1 {
		_ = call(errDependency)
	}

	if got := breaker.State(); got != resilience.BreakerClosed {
		t.Fatalf("State() = %v with %d failures, want closed below the floor of %d", got, floor-1, floor)
	}
}

func TestTheBreakerOpensAtTheFloorWithHalfTheCallsFailing(t *testing.T) {
	breaker, _, call := guarded(t, nil)
	floor := breakerPolicy().MinSamples

	for i := range floor {
		if i%2 == 0 {
			_ = call(errDependency)
		} else {
			_ = call(nil)
		}
	}

	if got := breaker.State(); got != resilience.BreakerOpen {
		t.Fatalf("State() = %v at %d samples and 50 %% failing, want open", got, floor)
	}
}

func TestAnOpenBreakerRefusesWithoutInvokingTheCall(t *testing.T) {
	fake := clock.NewFake(start)
	breaker := resilience.NewBreaker("payments", breakerPolicy(), fake, nil)

	invoked := 0
	decorated := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		invoked++
		return errDependency
	})
	for range breakerPolicy().MinSamples {
		_ = decorated(context.Background(), remoteOp(time.Second), nil)
	}
	opened := invoked

	err := decorated(context.Background(), remoteOp(time.Second), nil)

	if !errors.Is(err, resilience.ErrBreakerOpen) {
		t.Fatalf("decorated() = %v, want ErrBreakerOpen", err)
	}
	if invoked != opened {
		t.Fatalf("the call ran %d times after opening, want %d — an open breaker consumes neither timeout nor attempt (RES-12)", invoked, opened)
	}
}

func TestTheCooldownMovesTheBreakerToHalfOpenness(t *testing.T) {
	breaker, fake, call := guarded(t, nil)
	for range breakerPolicy().MinSamples {
		_ = call(errDependency)
	}

	fake.Advance(breakerPolicy().Cooldown - time.Nanosecond)
	if err := call(nil); !errors.Is(err, resilience.ErrBreakerOpen) {
		t.Fatalf("call() = %v one nanosecond before the cooldown, want ErrBreakerOpen", err)
	}

	fake.Advance(time.Nanosecond)
	if err := call(nil); err != nil {
		t.Fatalf("call() = %v at the cooldown, want the probe to go through", err)
	}
	if got := breaker.State(); got != resilience.BreakerClosed {
		t.Fatalf("State() = %v after a successful probe, want closed", got)
	}
}

func TestAFailingProbeReopensTheBreaker(t *testing.T) {
	breaker, fake, call := guarded(t, nil)
	for range breakerPolicy().MinSamples {
		_ = call(errDependency)
	}
	fake.Advance(breakerPolicy().Cooldown)

	if err := call(errDependency); !errors.Is(err, errDependency) {
		t.Fatalf("call() = %v, want the dependency error through the probe", err)
	}

	if got := breaker.State(); got != resilience.BreakerOpen {
		t.Fatalf("State() = %v after a failing probe, want open", got)
	}
	if err := call(nil); !errors.Is(err, resilience.ErrBreakerOpen) {
		t.Fatalf("call() = %v right after the probe failed, want ErrBreakerOpen: the cooldown restarts", err)
	}
}

func TestHalfOpennessAdmitsOnlyTheDeclaredProbes(t *testing.T) {
	fake := clock.NewFake(start)
	breaker := resilience.NewBreaker("payments", breakerPolicy(), fake, nil)

	release := make(chan struct{})
	entered := make(chan struct{}, 8)
	decorated := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		entered <- struct{}{}
		<-release
		return nil
	})

	// Open the breaker with calls that return at once.
	closing := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return errDependency
	})
	for range breakerPolicy().MinSamples {
		_ = closing(context.Background(), remoteOp(time.Second), nil)
	}
	fake.Advance(breakerPolicy().Cooldown)

	// The first probe blocks inside the call, holding half-openness.
	probe := make(chan error, 1)
	go func() { probe <- decorated(context.Background(), remoteOp(time.Second), nil) }()
	<-entered

	second := closing(context.Background(), remoteOp(time.Second), nil)
	if !errors.Is(second, resilience.ErrBreakerOpen) {
		t.Fatalf("the second call during half-openness = %v, want ErrBreakerOpen with one probe in flight", second)
	}

	close(release)
	if err := <-probe; err != nil {
		t.Fatalf("the probe = %v, want nil", err)
	}
	if got := breaker.State(); got != resilience.BreakerClosed {
		t.Fatalf("State() = %v, want closed after the probe succeeded", got)
	}
}

func TestFailuresOlderThanTheWindowAreForgotten(t *testing.T) {
	breaker, fake, call := guarded(t, nil)
	policy := breakerPolicy()

	for range policy.MinSamples - 1 {
		_ = call(errDependency)
	}
	fake.Advance(policy.Window + time.Nanosecond)
	for range policy.MinSamples - 1 {
		_ = call(errDependency)
	}

	if got := breaker.State(); got != resilience.BreakerClosed {
		t.Fatalf("State() = %v, want closed — the first burst aged out of the window (RES-10)", got)
	}
}

func TestACancellationByTheCallerIsNotHeldAgainstTheDependency(t *testing.T) {
	breaker, _, call := guarded(t, nil)

	for range breakerPolicy().MinSamples * 2 {
		_ = call(resilience.ErrCancelled)
	}

	if got := breaker.State(); got != resilience.BreakerClosed {
		t.Fatalf("State() = %v, want closed — the dependency was never given the chance to answer", got)
	}
}

func TestTheStateReachesTheGauge(t *testing.T) {
	instruments, read := meter(t)
	_, _, call := guarded(t, instruments)

	for range breakerPolicy().MinSamples {
		_ = call(errDependency)
	}

	if got := read(metrics.BreakerState); got != int64(resilience.BreakerOpen) {
		t.Fatalf("%s = %d, want %d (MET-28)", metrics.BreakerState, got, resilience.BreakerOpen)
	}
}

func TestTheStateNamesItself(t *testing.T) {
	want := map[resilience.BreakerState]string{
		resilience.BreakerClosed:   "closed",
		resilience.BreakerHalfOpen: "half_open",
		resilience.BreakerOpen:     "open",
	}
	for state, name := range want {
		if got := state.String(); got != name {
			t.Errorf("BreakerState(%d).String() = %q, want %q", state, got, name)
		}
	}
}

func TestTheBreakerIsSafeUnderConcurrentCalls(t *testing.T) {
	fake := clock.NewFake(start)
	breaker := resilience.NewBreaker("payments", breakerPolicy(), fake, nil)
	decorated := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return errDependency
	})

	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = decorated(context.Background(), remoteOp(time.Second), nil)
		}()
	}
	wg.Wait()

	if got := breaker.State(); got != resilience.BreakerOpen {
		t.Fatalf("State() = %v after 64 concurrent failures, want open", got)
	}
}

// A call admitted while the breaker was closed can still be in flight when the
// breaker opens, cools down and goes half-open. Its outcome belongs to the cycle
// it started in, and must not be taken for the probe's: a stale success would
// close the breaker over a dependency that never recovered.
func TestACallFromBeforeTheCycleDoesNotDecideTheHalfOpenOutcome(t *testing.T) {
	fake := clock.NewFake(start)
	policy := breakerPolicy()
	breaker := resilience.NewBreaker("payments", policy, fake, nil)

	entered := make(chan struct{})
	release := make(chan struct{})

	// The straggler: admitted while closed, held until the test says otherwise.
	straggler := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		close(entered)
		<-release
		return nil // it succeeds, which is what would wrongly close the breaker
	})

	var inFlight sync.WaitGroup
	inFlight.Add(1)
	go func() {
		defer inFlight.Done()
		_ = straggler(context.Background(), remoteOp(time.Second), nil)
	}()
	<-entered

	// Meanwhile the dependency fails enough to open the breaker.
	failing := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return errDependency
	})
	for range policy.MinSamples {
		_ = failing(context.Background(), remoteOp(time.Second), nil)
	}
	if got := breaker.State(); got != resilience.BreakerOpen {
		t.Fatalf("State() = %v, want open before the cooldown", got)
	}

	// The cooldown elapses and a probe is admitted, which is what moves the
	// breaker to half-openness.
	fake.Advance(policy.Cooldown + time.Second)
	probeEntered := make(chan struct{})
	probeRelease := make(chan struct{})
	probe := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		close(probeEntered)
		<-probeRelease
		return errDependency
	})
	var probing sync.WaitGroup
	probing.Add(1)
	go func() {
		defer probing.Done()
		_ = probe(context.Background(), remoteOp(time.Second), nil)
	}()
	<-probeEntered

	if got := breaker.State(); got != resilience.BreakerHalfOpen {
		t.Fatalf("State() = %v, want half-open once a probe is in flight", got)
	}

	// Now the straggler finishes, successfully. It started two cycles ago.
	close(release)
	inFlight.Wait()

	if got := breaker.State(); got != resilience.BreakerHalfOpen {
		t.Errorf("State() = %v, want it to stay half-open: a success from before the cycle closed the breaker", got)
	}

	// The real probe fails, and that is what decides.
	close(probeRelease)
	probing.Wait()

	if got := breaker.State(); got != resilience.BreakerOpen {
		t.Errorf("State() = %v, want open: the probe failed, and the probe is what decides", got)
	}
}

// A cancelled call releases a probe slot only if it held one. A call admitted
// while the breaker was closed never incremented the counter, and decrementing
// it on the way out would leave the half-open cycle admitting more probes than
// the policy allows.
func TestACancelledCallReleasesOnlyTheProbeSlotItHeld(t *testing.T) {
	fake := clock.NewFake(start)
	policy := breakerPolicy()
	breaker := resilience.NewBreaker("payments", policy, fake, nil)

	cancelled := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return resilience.ErrCancelled
	})

	// Closed breaker: these calls never held a probe slot.
	for range 4 {
		_ = cancelled(context.Background(), remoteOp(time.Second), nil)
	}

	// A cancellation is not held against the dependency, so the breaker is still
	// closed and no probe accounting happened at all.
	if got := breaker.State(); got != resilience.BreakerClosed {
		t.Fatalf("State() = %v, want closed — a caller giving up is not a dependency failing", got)
	}

	// Open it, cool it down, and check the half-open cycle still admits exactly
	// the declared number of probes.
	failing := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return errDependency
	})
	for range policy.MinSamples {
		_ = failing(context.Background(), remoteOp(time.Second), nil)
	}
	fake.Advance(policy.Cooldown + time.Second)

	entered := make(chan struct{})
	release := make(chan struct{})
	probe := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		close(entered)
		<-release
		return errDependency
	})
	var probing sync.WaitGroup
	probing.Add(1)
	go func() {
		defer probing.Done()
		_ = probe(context.Background(), remoteOp(time.Second), nil)
	}()
	<-entered

	// With the single declared probe in flight, the next call is refused.
	if err := failing(context.Background(), remoteOp(time.Second), nil); !errors.Is(err, resilience.ErrBreakerOpen) {
		t.Errorf("second call during half-openness = %v, want ErrBreakerOpen: the probe slot is taken", err)
	}

	close(release)
	probing.Wait()
}

// A probe that is cancelled gives its slot back, so the half-open cycle can
// still be decided by a probe that actually reaches the dependency.
func TestACancelledProbeGivesItsSlotBack(t *testing.T) {
	fake := clock.NewFake(start)
	policy := breakerPolicy()
	breaker := resilience.NewBreaker("payments", policy, fake, nil)

	failing := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return errDependency
	})
	for range policy.MinSamples {
		_ = failing(context.Background(), remoteOp(time.Second), nil)
	}
	fake.Advance(policy.Cooldown + time.Second)

	// The first probe is cancelled by its caller, which is not the dependency
	// failing: it releases the slot and decides nothing.
	cancelledProbe := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return resilience.ErrCancelled
	})
	if err := cancelledProbe(context.Background(), remoteOp(time.Second), nil); !errors.Is(err, resilience.ErrCancelled) {
		t.Fatalf("probe = %v, want the cancellation", err)
	}
	if got := breaker.State(); got != resilience.BreakerHalfOpen {
		t.Fatalf("State() = %v, want half-open: a cancelled probe decides nothing", got)
	}

	// The slot is free, so a real probe gets through and closes the breaker.
	succeeding := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return nil
	})
	if err := succeeding(context.Background(), remoteOp(time.Second), nil); err != nil {
		t.Fatalf("second probe = %v, want nil: the cancelled probe freed its slot", err)
	}
	if got := breaker.State(); got != resilience.BreakerClosed {
		t.Errorf("State() = %v, want closed: the probe succeeded", got)
	}
}

var errAnswer = errors.New("payments: 404 not found")

func classified(t *testing.T) (*resilience.Breaker, *clock.Fake, func(err error) error) {
	t.Helper()

	fake := clock.NewFake(start)
	breaker := resilience.NewBreaker("payments", breakerPolicy(), fake, nil).
		CountsAsFailure(func(err error) bool { return !errors.Is(err, errAnswer) })

	var outcome error
	decorated := breaker.Decorate()(func(context.Context, resilience.Operation, func(context.Context) error) error {
		return outcome
	})

	return breaker, fake, func(err error) error {
		outcome = err
		return decorated(context.Background(), remoteOp(time.Second), nil)
	}
}

func TestAnErrorTheClassifierDoesNotCountNeverOpensTheBreaker(t *testing.T) {
	breaker, _, call := classified(t)
	floor := breakerPolicy().MinSamples

	for range floor * 2 {
		if err := call(errAnswer); !errors.Is(err, errAnswer) {
			t.Fatalf("call() = %v, want the answer through", err)
		}
	}
	if got := breaker.State(); got != resilience.BreakerClosed {
		t.Fatalf("State() = %v after %d uncounted errors, want closed — an answer is not unavailability (RES-10)", got, floor*2)
	}

	for range floor * 3 {
		_ = call(errDependency)
	}
	if got := breaker.State(); got != resilience.BreakerOpen {
		t.Fatalf("State() = %v after %d counted failures, want open", got, floor*3)
	}
}

func TestAProbeAnsweredWithAnUncountedErrorClosesTheBreaker(t *testing.T) {
	breaker, fake, call := classified(t)
	for range breakerPolicy().MinSamples {
		_ = call(errDependency)
	}
	fake.Advance(breakerPolicy().Cooldown)

	if err := call(errAnswer); !errors.Is(err, errAnswer) {
		t.Fatalf("call() = %v, want the answer through the probe", err)
	}
	if got := breaker.State(); got != resilience.BreakerClosed {
		t.Fatalf("State() = %v after a probe the dependency answered, want closed", got)
	}
}
