// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-08 (TRC-12, MET-07) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfkafka

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/twmb/franz-go/pkg/kerr"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/observe"
)

// Classifier is the retry taxonomy of the producer: a broker error Kafka itself
// calls retriable and a network failure are worth another attempt; a
// cancellation, a deadline and any other error are not.
func Classifier(err error) retry.Retryability {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return retry.NotRetryable
	}
	var broker *kerr.Error
	if errors.As(err, &broker) {
		if broker.Retriable {
			return retry.Retryable
		}
		return retry.NotRetryable
	}
	var network net.Error
	if errors.As(err, &network) {
		return retry.Retryable
	}
	return retry.NotRetryable
}

// categoryOf returns the bounded category under which a failure is recorded:
// a platform category, the Kafka error code name in lowercase, the context
// error or "network" — never the message (TRC-12, MET-07).
func categoryOf(err error) string {
	if err == nil {
		return observe.CategoryOK
	}
	var categorized interface{ ErrorCategory() string }
	if errors.As(err, &categorized) {
		return categorized.ErrorCategory()
	}
	var broker *kerr.Error
	if errors.As(err, &broker) {
		return strings.ToLower(broker.Message)
	}
	switch {
	case errors.Is(err, ErrSinkPanicked):
		return "panic"
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
