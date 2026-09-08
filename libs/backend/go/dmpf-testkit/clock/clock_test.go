package clock_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	obsclock "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/clock"
)

var (
	_ dmpfports.Clock = (*clock.Fake)(nil)
	_ obsclock.Clock  = clock.New(0).Observability()
)

const start = dmpfports.Instant(1_700_000_000_000_000_000)

func TestNowReturnsTheInstantItWasGiven(t *testing.T) {
	c := clock.New(start)
	if got := c.Now(); got != start {
		t.Fatalf("Now() = %d, want %d", got, start)
	}
	if got := c.Now(); got != start {
		t.Fatalf("second Now() = %d, want %d: the fake moved on its own", got, start)
	}
}

func TestAdvanceMovesByTheDuration(t *testing.T) {
	c := clock.New(start)
	c.Advance(2 * time.Second)
	if got, want := c.Now(), start+2_000_000_000; got != want {
		t.Fatalf("Now() after Advance = %d, want %d", got, want)
	}
	c.Advance(-time.Second)
	if got, want := c.Now(), start+2_000_000_000; got != want {
		t.Fatalf("negative Advance moved the clock to %d, want %d", got, want)
	}
}

func TestSetMovesForwardAndBackward(t *testing.T) {
	c := clock.New(start)
	c.Set(start + 10)
	if got := c.Now(); got != start+10 {
		t.Fatalf("Set forward: Now() = %d, want %d", got, start+10)
	}
	c.Set(start - 10)
	if got := c.Now(); got != start-10 {
		t.Fatalf("Set backward: Now() = %d, want %d", got, start-10)
	}
	if got := c.Observability().Now().UnixNano(); got != int64(start-10) {
		t.Fatalf("Observability().Now() = %d after Set backward, want %d", got, start-10)
	}
}

func TestObservabilityViewSharesTheInstant(t *testing.T) {
	c := clock.New(start)
	view := c.Observability()
	if got := view.Now().UnixNano(); got != int64(start) {
		t.Fatalf("view.Now() = %d, want %d", got, start)
	}
	c.Advance(time.Minute)
	if got := view.Now().UnixNano(); got != int64(start)+int64(time.Minute) {
		t.Fatalf("view.Now() after Advance = %d, want %d", got, int64(start)+int64(time.Minute))
	}
}

func TestObservabilityTimersFireOnAdvance(t *testing.T) {
	c := clock.New(start)
	view := c.Observability()
	after := view.After(time.Second)
	timer := view.NewTimer(2 * time.Second)
	ctx, cancel := view.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	select {
	case <-after:
		t.Fatal("After fired before Advance")
	default:
	}

	c.Advance(time.Second)
	if _, ok := <-after; !ok {
		t.Fatal("After did not fire at its instant")
	}
	select {
	case <-timer.C():
		t.Fatal("timer fired one second early")
	default:
	}

	c.Advance(time.Second)
	<-timer.C()
	if ctx.Err() != nil {
		t.Fatalf("context expired one second early: %v", ctx.Err())
	}

	c.Advance(time.Second)
	<-ctx.Done()
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("ctx.Err() = %v, want DeadlineExceeded", ctx.Err())
	}
}

func TestSetBackwardRefusesPendingTimers(t *testing.T) {
	c := clock.New(start)
	view := c.Observability()
	after := view.After(time.Second)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Set backward with a pending timer did not panic")
		}
		if got := fmt.Sprint(r); !strings.Contains(got, "1 pending timer") {
			t.Fatalf("panic = %q, want it to count the pending timer", got)
		}
		c.Advance(2 * time.Second)
		select {
		case <-after:
		default:
			t.Fatal("the refused Set must leave the timer intact; it did not fire")
		}
	}()
	c.Set(start - 1)
}
