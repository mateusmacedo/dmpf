package app

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ErrInvalidIdempotencyPolicy is IdempotencyPolicy's refusal of a wait or a
// retention that would let a retry run the command again.
var ErrInvalidIdempotencyPolicy = errors.New("app: invalid idempotency policy")

// IdempotencyPolicy is the policy a context runs its commands under: the wait
// ceiling (IDM-07), the retention of each entry (IDM-09) and SHA-256 over the
// canonical fingerprint (IDM-04).
func IdempotencyPolicy(wait, retention time.Duration) (application.IdempotencyPolicy, error) {
	if wait <= 0 || retention <= 0 {
		return application.IdempotencyPolicy{}, fmt.Errorf("%w: wait %v, retention %v", ErrInvalidIdempotencyPolicy, wait, retention)
	}
	return application.IdempotencyPolicy{Wait: int64(wait), Retention: int64(retention), Digest: fingerprintDigest}, nil
}

func fingerprintDigest(canonical []byte) ports.Fingerprint {
	return sha256.Sum256(canonical)
}
