// comment-discipline-ok-file: arquivo de declarações da composition root; cada godoc é contrato de API pública com referência normativa (RFC §9.3, OBX-08, FND-04 §4.1), dentro do limite de 3 linhas.

package dmpfreferencereservations

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

const identityBytes = 16

// SystemClock is the wall clock. It lives in the composition root because
// reading the clock is io.clock capability (RFC §9.3).
type SystemClock struct{}

func (SystemClock) Now() dmpfports.Instant { return dmpfports.Instant(time.Now().UnixNano()) }

// RandomClaimIDs mints a claim identity per acquisition (OBX-08).
type RandomClaimIDs struct{}

func (RandomClaimIDs) NewClaimID() string { return randomHex() }

// RandomMessageIDs mints the message_id of FND-04 §4.1 before the transaction opens.
type RandomMessageIDs struct{}

func (RandomMessageIDs) NewMessageID() dmpfports.MessageID { return dmpfports.MessageID(randomHex()) }

// randomHex panics only if the operating system's entropy source fails, which
// crypto/rand documents as unrecoverable.
func randomHex() string {
	buffer := make([]byte, identityBytes)
	if _, err := rand.Read(buffer); err != nil {
		panic("dmpf-reference-reservations: the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}
