// Package clock is the provider's time: reading the instant, waiting, timers and
// derived deadlines, plus a deterministic fake that tests advance by hand.
//
// It lives here because ports classifies the whole time package as
// io.clock and cannot import it, so the blocks above receive the instant
// through a port and the realization is this one.
package clock
