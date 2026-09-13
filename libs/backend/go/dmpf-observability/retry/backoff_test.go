package retry_test

import (
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/retry"
)

func TestTheIntervalDoublesPerAttempt(t *testing.T) {
	backoff := retry.DefaultBackoff()
	whole := func() float64 { return 1 }

	cases := map[int]time.Duration{
		0: 100 * time.Millisecond,
		1: 200 * time.Millisecond,
		2: 400 * time.Millisecond,
		3: 800 * time.Millisecond,
		4: 1600 * time.Millisecond,
		5: 3200 * time.Millisecond,
	}
	for attempt, want := range cases {
		if got := backoff.Next(attempt, whole); got != want {
			t.Errorf("Next(%d) = %v, want %v", attempt, got, want)
		}
	}
}

func TestTheIntervalStopsAtTheCap(t *testing.T) {
	backoff := retry.DefaultBackoff()
	whole := func() float64 { return 1 }

	for _, attempt := range []int{6, 7, 20, 1000} {
		if got := backoff.Next(attempt, whole); got != retry.DefaultCap {
			t.Errorf("Next(%d) = %v, want the cap %v", attempt, got, retry.DefaultCap)
		}
	}
}

func TestTheJitterIsFullAndDrawnFromTheInjectedSource(t *testing.T) {
	backoff := retry.DefaultBackoff()

	cases := []struct {
		draw float64
		want time.Duration
	}{
		{draw: 0, want: 0},
		{draw: 0.5, want: 100 * time.Millisecond},
		{draw: 1, want: 200 * time.Millisecond},
	}
	for _, caso := range cases {
		got := backoff.Next(1, func() float64 { return caso.draw })
		if got != caso.want {
			t.Errorf("Next(1) with draw %v = %v, want %v", caso.draw, got, caso.want)
		}
	}
}

func TestADrawOutsideTheUnitIntervalIsClamped(t *testing.T) {
	backoff := retry.DefaultBackoff()

	if got := backoff.Next(0, func() float64 { return -3 }); got != 0 {
		t.Errorf("Next() with a negative draw = %v, want 0", got)
	}
	if got := backoff.Next(0, func() float64 { return 7 }); got != retry.DefaultBase {
		t.Errorf("Next() with a draw above 1 = %v, want the whole interval %v", got, retry.DefaultBase)
	}
}

func TestANilRandWaitsTheWholeInterval(t *testing.T) {
	backoff := retry.DefaultBackoff()

	if got := backoff.Next(1, nil); got != 200*time.Millisecond {
		t.Fatalf("Next(1, nil) = %v, want the whole interval: an absent source of randomness is read conservatively", got)
	}
}

func TestAZeroBaseNeverWaits(t *testing.T) {
	backoff := retry.Backoff{Factor: 2, Cap: retry.DefaultCap}

	if got := backoff.Next(3, func() float64 { return 1 }); got != 0 {
		t.Fatalf("Next() = %v, want 0 with no base", got)
	}
}

func TestAFactorOfOneKeepsTheBaseInterval(t *testing.T) {
	backoff := retry.Backoff{Base: retry.DefaultBase, Factor: 1, Cap: retry.DefaultCap}

	if got := backoff.Next(5, func() float64 { return 1 }); got != retry.DefaultBase {
		t.Fatalf("Next(5) = %v, want the base %v with factor 1", got, retry.DefaultBase)
	}
}

func TestANegativeAttemptNeverWaits(t *testing.T) {
	backoff := retry.DefaultBackoff()

	if got := backoff.Next(-1, func() float64 { return 1 }); got != 0 {
		t.Fatalf("Next(-1) = %v, want 0", got)
	}
}
