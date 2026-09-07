// Package clock is the fake realization of dmpfports.Clock the kits inject so
// no scenario reads the wall clock (KIT-07): the instant advances only when the
// test says so. Provider block.
package clock
