// comment-discipline-ok-file: arquivo de declarações da composition root; cada godoc é contrato de API pública com referência normativa (RFC §9.3, OBX-08, FND-04 §4.1), dentro do limite de 3 linhas.

package orders

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const identityBytes = 16

// SystemClock is the wall clock. It lives in the composition root because
// reading the clock is io.clock capability (RFC §9.3).
type SystemClock struct{}

func (SystemClock) Now() ports.Instant { return ports.Instant(time.Now().UnixNano()) }

// RandomClaimIDs mints a claim identity per acquisition (OBX-08).
type RandomClaimIDs struct{}

func (RandomClaimIDs) NewClaimID() string { return randomHex() }

// RandomMessageIDs mints the message_id of FND-04 §4.1 before the transaction opens.
type RandomMessageIDs struct{}

func (RandomMessageIDs) NewMessageID() ports.MessageID { return ports.MessageID(randomHex()) }

// randomHex panics only if the operating system's entropy source fails, which
// crypto/rand documents as unrecoverable.
func randomHex() string {
	buffer := make([]byte, identityBytes)
	if _, err := rand.Read(buffer); err != nil {
		panic("orders: the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}
