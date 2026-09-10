// comment-discipline-ok-file: arquivo de declarações da composition root; cada godoc é contrato de API pública com referência normativa (RFC §9.3, OBX-08, FND-04 §4.1), dentro do limite de 3 linhas.

package dmpfreference

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// identityBytes is the width of a minted identity: 128 bits of entropy settle
// uniqueness without a coordinator, for claims (OBX-08) and messages alike.
const identityBytes = 16

// SystemClock is the wall clock. It lives in the composition root because
// reading the clock is io.clock capability: the blocks above receive the
// instant instead of asking for it (RFC §9.3).
type SystemClock struct{}

func (SystemClock) Now() dmpfports.Instant { return dmpfports.Instant(time.Now().UnixNano()) }

// RandomClaimIDs mints a claim identity per acquisition (OBX-08). Entropy is
// io.random capability, which is why it enters through a port.
type RandomClaimIDs struct{}

func (RandomClaimIDs) NewClaimID() string { return randomHex() }

// RandomMessageIDs mints the message_id of FND-04 §4.1 before the transaction
// opens; the application service never reads entropy itself (RFC §9.3).
type RandomMessageIDs struct{}

func (RandomMessageIDs) NewMessageID() dmpfports.MessageID { return dmpfports.MessageID(randomHex()) }

// randomHex panics only if the operating system's entropy source fails, which
// crypto/rand documents as unrecoverable: a process that cannot tell its
// identities apart must not keep minting them.
func randomHex() string {
	buffer := make([]byte, identityBytes)
	if _, err := rand.Read(buffer); err != nil {
		panic("dmpf-reference: the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}
