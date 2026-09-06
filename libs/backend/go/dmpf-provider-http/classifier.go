// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (RST-02) ou FND-08 (TRC-12, MET-07) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfhttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/observe"
)

// RetryableStatusError is a response whose status the route declared
// transient, turned into an error so the retry conjunction can decide on it;
// the response is kept, so the last one is what the caller gets back.
type RetryableStatusError struct {
	Status   int
	Response *http.Response
}

func (e *RetryableStatusError) Error() string {
	return fmt.Sprintf("dmpfhttp: retryable status %d", e.Status)
}

// Classifier is the retry taxonomy of the HTTP transport (RST-02): a declared
// transient status and a network failure are worth another attempt; a
// cancellation, a deadline and any other error are not.
func Classifier(err error) retry.Retryability {
	var transient *RetryableStatusError
	if errors.As(err, &transient) {
		return retry.Retryable
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return retry.NotRetryable
	}
	var network net.Error
	if errors.As(err, &network) {
		return retry.Retryable
	}
	return retry.NotRetryable
}

// categoryOf returns the bounded category under which a failure is recorded:
// a platform category, the transient status, the context error or "network" —
// never the message (TRC-12, MET-07).
func categoryOf(err error) string {
	if err == nil {
		return observe.CategoryOK
	}
	var categorized interface{ ErrorCategory() string }
	if errors.As(err, &categorized) {
		return categorized.ErrorCategory()
	}
	var transient *RetryableStatusError
	if errors.As(err, &transient) {
		return fmt.Sprintf("status_%d", transient.Status)
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	}
	var network net.Error
	if errors.As(err, &network) {
		return "network"
	}
	return "unknown"
}
