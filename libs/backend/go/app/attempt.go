// comment-discipline-ok-file: arquivo de declarações do adapter; cada godoc é contrato de API pública com referência normativa (CTX-28), dentro do limite de 3 linhas.

package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

const attemptIDBytes = 16

// Attempt is what the adapter mints for one processing attempt: an identifier
// of its own and the transport's delivery count. Neither is read from the
// envelope, because the consumption is the consumer's execution (CTX-28).
type Attempt struct {
	RequestID string
	Number    int
}

type attemptKey struct{}

// WithAttempt carries the attempt to the handler, which is the only channel the
// adapter has between the two.
func WithAttempt(ctx context.Context, attempt Attempt) context.Context {
	return context.WithValue(ctx, attemptKey{}, attempt)
}

// AttemptFrom returns what the adapter minted. Only the handler reads it, to
// mount the execution context; from there down the context travels as an
// explicit argument (CTX-03).
func AttemptFrom(ctx context.Context) (Attempt, bool) {
	attempt, ok := ctx.Value(attemptKey{}).(Attempt)
	return attempt, ok
}

func newAttemptID() string {
	buffer := make([]byte, attemptIDBytes)
	if _, err := rand.Read(buffer); err != nil {
		panic("app: the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}
