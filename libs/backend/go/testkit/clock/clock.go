package clock

import (
	"context"
	"strconv"
	"sync"
	"time"

	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Fake realizes ports.Clock over one obsclock.Fake, so the kernel's instant
// and the transport's timers never disagree: Advance moves both at once.
type Fake struct {
	mu  sync.Mutex
	obs *obsclock.Fake
}

// New starts the fake at the given instant. Zero is accepted: a deterministic
// test cares about differences, not about the date.
func New(at ports.Instant) *Fake {
	return &Fake{obs: obsclock.NewFake(time.Unix(0, int64(at)))}
}

func (f *Fake) Now() ports.Instant {
	return ports.Instant(f.current().Now().UnixNano())
}

// Advance moves the clock forward and fires every observability timer due at
// the new instant. Non-positive durations do nothing.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.obs.Advance(d)
}

// Set moves the clock to an absolute instant. Moving forward, it behaves as
// Advance; moving backward, it replaces the underlying fake, which is only possible while no
// timer is pending — a timer scheduled in a future that no longer exists would
// never fire, and whoever waits on it would hang without a diagnosis.
func (f *Fake) Set(at ports.Instant) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.obs.Now().UnixNano()
	if delta := int64(at) - now; delta >= 0 {
		f.obs.Advance(time.Duration(delta))
		return
	}
	if n := f.obs.Pending(); n != 0 {
		panic("clock: Set backward with " + strconv.Itoa(n) + " pending timers; fire or stop them first")
	}
	f.obs = obsclock.NewFake(time.Unix(0, int64(at)))
}

// Observability is the same instant seen as obsclock.Clock, for the providers
// of KRN-09/KRN-10 that wait on timers and contexts rather than read a value.
func (f *Fake) Observability() obsclock.Clock { return view{f} }

func (f *Fake) current() *obsclock.Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.obs
}

type view struct{ f *Fake }

func (v view) Now() time.Time                          { return v.f.current().Now() }
func (v view) After(d time.Duration) <-chan time.Time  { return v.f.current().After(d) }
func (v view) NewTimer(d time.Duration) obsclock.Timer { return v.f.current().NewTimer(d) }
func (v view) WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return v.f.current().WithTimeout(ctx, d)
}
