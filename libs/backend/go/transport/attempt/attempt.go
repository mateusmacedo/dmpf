// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (TRP-52, TRP-18) que o símbolo realiza, dentro do limite de 3 linhas.

// Package attempt is the attempt count carried as message metadata by a
// transport that keeps no count of its own (TRP-52), beside the envelope and
// never inside it (TRP-18).
package attempt

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

// Header is the header or message-attribute key the count travels under.
const Header = "dmpf-attempt"

// ErrInvalidAttempt is a value that is not a positive integer: an attempt
// count starts at one.
var ErrInvalidAttempt = errors.New("attempt: value is not a positive integer")

// Encode renders the attempt count for the header.
func Encode(n int) string { return strconv.Itoa(n) }

// Decode parses the header value, refusing anything that is not a positive
// integer.
func Decode(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidAttempt, value)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%w: %d", ErrInvalidAttempt, n)
	}
	return n, nil
}

type contextKey struct{}

// WithContext records the attempt of the delivery in flight, so a containment
// reached through the same context can carry it beside the envelope (TRP-52).
func WithContext(ctx context.Context, attempt int) context.Context {
	return context.WithValue(ctx, contextKey{}, attempt)
}

// FromContext reads the attempt recorded by WithContext; false when none is.
func FromContext(ctx context.Context) (int, bool) {
	attempt, ok := ctx.Value(contextKey{}).(int)
	return attempt, ok && attempt > 0
}
