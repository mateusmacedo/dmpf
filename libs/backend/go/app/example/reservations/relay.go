// comment-discipline-ok-file: arquivo de declarações da composition root; cada godoc é contrato de API pública com referência normativa (FND-04 §5.4, OBX-08, BLK-02), dentro do limite de 3 linhas.

package reservationsconsumer

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// claimIDBytes is the width of a claim identity. OBX-08 asks only that two
// acquisitions never collide, and 128 bits of entropy settle that without a
// coordinator.
const claimIDBytes = 16

// RandomClaimIDs mints a claim identity per acquisition. Entropy is io.random
// capability, which is why it enters through a port instead of being read
// wherever the claim happens to run.
type RandomClaimIDs struct{}

// NewClaimID panics only if the operating system's entropy source fails, which
// crypto/rand documents as unrecoverable: a relay that cannot tell its claims
// apart must not keep claiming.
func (RandomClaimIDs) NewClaimID() string {
	buffer := make([]byte, claimIDBytes)
	if _, err := rand.Read(buffer); err != nil {
		panic("relay: the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}

// SystemClock is the wall clock. It lives in the composition root because
// reading the clock is io.clock capability: the blocks above receive the
// instant instead of asking for it (RFC §9.3).
type SystemClock struct{}

func (SystemClock) Now() ports.Instant { return ports.Instant(time.Now().UnixNano()) }

// NewRelay assembles the drain over Postgres. It runs in a process of its own,
// separate from the one serving requests: there, broker latency would compete
// with the request path (BLK-02).
func NewRelay(pool *pgxpool.Pool, publisher relay.Publisher, config relay.Config) (relay.Relay, error) {
	clock := SystemClock{}
	return relay.New(postgres.NewOutboxStore(pool, clock), publisher, RandomClaimIDs{}, clock, config)
}
