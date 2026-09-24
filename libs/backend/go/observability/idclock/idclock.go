// Package idclock realizes the domain ports over the operating system: the wall
// clock behind ports.Clock and the identities minted before a transaction opens.
//
// It is not the clock of the sibling package clock, which serves timers,
// deadlines and waiting in terms of time.Time. Here the instant is the
// ports.Instant of the domain — nanoseconds as an integer — because ports
// classifies the whole time package as io.clock and cannot import it.
package idclock

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const identityBytes = 16

// SystemClock is the wall clock. It lives in a provider because reading the
// clock is io.clock capability (RFC §9.3).
type SystemClock struct{}

func (SystemClock) Now() ports.Instant { return ports.Instant(time.Now().UnixNano()) }

// NewClaimIDs takes the component that names the process because the only
// failure here is unrecoverable, and the panic must say who was minting.
func NewClaimIDs(component string) RandomClaimIDs { return RandomClaimIDs{component: component} }

// RandomClaimIDs mints a claim identity per acquisition (OBX-08).
type RandomClaimIDs struct{ component string }

func (r RandomClaimIDs) NewClaimID() string { return randomHex(r.component) }

// NewMessageIDs takes the component for the same reason as NewClaimIDs.
func NewMessageIDs(component string) RandomMessageIDs {
	return RandomMessageIDs{component: component}
}

// RandomMessageIDs mints the message_id of FND-04 §4.1 before the transaction opens.
type RandomMessageIDs struct{ component string }

func (r RandomMessageIDs) NewMessageID() ports.MessageID {
	return ports.MessageID(randomHex(r.component))
}

// randomHex panics only if the operating system's entropy source fails, which
// crypto/rand documents as unrecoverable.
func randomHex(component string) string {
	buffer := make([]byte, identityBytes)
	if _, err := rand.Read(buffer); err != nil {
		panic(component + ": the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}
