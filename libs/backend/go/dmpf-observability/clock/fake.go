package clock

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"
)

// Fake is a clock the test advances by hand. Nothing fires on its own, so a
// suite never waits on wall time and never races the scheduler.
//
// Safe for concurrent use: the decorators under test wait on it from several
// goroutines while the test advances it from one.
type Fake struct {
	mu      sync.Mutex
	now     time.Time
	pending []*alarm
	nextSeq uint64
}

// alarm carries seq so two alarms due at the same instant fire in the order
// they were scheduled. The counter is monotonic and never reused, because the
// pending slice shrinks and its length would collide.
type alarm struct {
	at    time.Time
	seq   uint64
	fire  func(now time.Time)
	fired bool
}

// NewFake starts at the given instant. The zero instant is accepted: what
// matters to a deterministic test is the difference, not the absolute date.
func NewFake(start time.Time) *Fake { return &Fake{now: start} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the clock and fires every alarm due at the new instant, in
// deadline order. An alarm scheduled by one that just fired is due on the next
// Advance, so a single call cannot cascade without bound.
func (f *Fake) Advance(d time.Duration) {
	if d <= 0 {
		return
	}

	f.mu.Lock()
	f.now = f.now.Add(d)
	now := f.now

	due := make([]*alarm, 0, len(f.pending))
	kept := make([]*alarm, 0, len(f.pending))
	for _, a := range f.pending {
		switch {
		case a.fired:
			continue
		case a.at.After(now):
			kept = append(kept, a)
		default:
			a.fired = true
			due = append(due, a)
		}
	}
	f.pending = kept
	f.mu.Unlock()

	slices.SortStableFunc(due, func(a, b *alarm) int {
		if !a.at.Equal(b.at) {
			return a.at.Compare(b.at)
		}
		return cmp.Compare(a.seq, b.seq)
	})
	for _, a := range due {
		a.fire(now)
	}
}

// Pending is how many alarms are still waiting, so a test asserts that a
// decorator released its timer instead of leaking it.
func (f *Fake) Pending() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	waiting := 0
	for _, a := range f.pending {
		if !a.fired {
			waiting++
		}
	}
	return waiting
}

func (f *Fake) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	f.schedule(d, func(now time.Time) { ch <- now })
	return ch
}

func (f *Fake) NewTimer(d time.Duration) Timer {
	t := &fakeTimer{clock: f, ch: make(chan time.Time, 1)}
	t.alarm = f.schedule(d, func(now time.Time) { t.ch <- now })
	return t
}

func (f *Fake) WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	f.mu.Lock()
	deadline := f.now.Add(d)
	f.mu.Unlock()

	ctx := &fakeContext{Context: parent, deadline: deadline, done: make(chan struct{})}
	a := f.schedule(d, func(time.Time) { ctx.close(context.DeadlineExceeded) })

	stop := make(chan struct{})
	go func() {
		select {
		case <-parent.Done():
			ctx.close(parent.Err())
		case <-ctx.done:
		case <-stop:
		}
	}()

	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			f.cancel(a)
			close(stop)
			ctx.close(context.Canceled)
		})
	}
}

// schedule registers an alarm. A non-positive duration is already due and fires
// outside the lock, so the callback cannot deadlock against the clock.
func (f *Fake) schedule(d time.Duration, fire func(now time.Time)) *alarm {
	f.mu.Lock()
	f.nextSeq++
	a := &alarm{at: f.now.Add(d), seq: f.nextSeq, fire: fire}
	overdue := d <= 0
	if overdue {
		a.fired = true
	} else {
		f.pending = append(f.pending, a)
	}
	now := f.now
	f.mu.Unlock()

	if overdue {
		fire(now)
	}
	return a
}

func (f *Fake) cancel(a *alarm) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if a.fired {
		return false
	}
	a.fired = true
	return true
}

// reschedule moves an alarm to a new deadline, reporting whether it stopped a
// pending expiry. An alarm that had not fired is still in the queue, so it is
// updated in place instead of enqueued twice.
func (f *Fake) reschedule(a *alarm, d time.Duration) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	stopped := !a.fired
	a.at = f.now.Add(d)
	f.nextSeq++
	a.seq = f.nextSeq

	if a.fired {
		a.fired = false
		f.pending = append(f.pending, a)
	}
	return stopped
}

type fakeTimer struct {
	clock *Fake
	alarm *alarm
	ch    chan time.Time
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }

func (t *fakeTimer) Stop() bool { return t.clock.cancel(t.alarm) }

func (t *fakeTimer) Reset(d time.Duration) bool { return t.clock.reschedule(t.alarm, d) }

type fakeContext struct {
	context.Context

	deadline time.Time
	done     chan struct{}

	mu  sync.Mutex
	err error
}

func (c *fakeContext) Deadline() (time.Time, bool) { return c.deadline, true }

func (c *fakeContext) Done() <-chan struct{} { return c.done }

func (c *fakeContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *fakeContext) close(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.err != nil {
		return
	}
	c.err = err
	close(c.done)
}
