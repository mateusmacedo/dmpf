package resilience_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
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
