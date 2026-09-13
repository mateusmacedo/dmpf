package retry

import "time"

// DefaultBase, DefaultFactor and DefaultCap are the platform values of RES-32.
const (
	DefaultBase   = 100 * time.Millisecond
	DefaultFactor = 2
	DefaultCap    = 5 * time.Second
)

// Backoff is exponential with full jitter: the interval doubles per attempt up
// to Cap, and the wait is drawn uniformly below it, so retries of concurrent
// callers spread instead of arriving together.
type Backoff struct {
	Base   time.Duration
	Factor int
	Cap    time.Duration
}

// DefaultBackoff is the platform backoff of RES-32: 100 ms, factor 2, 5 s cap.
func DefaultBackoff() Backoff {
	return Backoff{Base: DefaultBase, Factor: DefaultFactor, Cap: DefaultCap}
}

// Next is the wait before the attempt that follows attempt, drawn from
// [0, interval) with the injected rand, so a test fixes the draw instead of
// tolerating a range. A nil rand waits the whole interval, which is the
// conservative reading of an absent source of randomness.
func (b Backoff) Next(attempt int, rand func() float64) time.Duration {
	interval := b.interval(attempt)
	if interval <= 0 {
		return 0
	}
	if rand == nil {
		return interval
	}

	draw := rand()
	switch {
	case draw <= 0:
		return 0
	case draw >= 1:
		return interval
	default:
		return time.Duration(float64(interval) * draw)
	}
}

func (b Backoff) interval(attempt int) time.Duration {
	if b.Base <= 0 || attempt < 0 {
		return 0
	}

	interval := b.Base
	for range attempt {
		if b.Factor <= 1 {
			break
		}
		interval *= time.Duration(b.Factor)
		if b.Cap > 0 && interval >= b.Cap {
			return b.Cap
		}
	}
	if b.Cap > 0 && interval > b.Cap {
		return b.Cap
	}
	return interval
}
