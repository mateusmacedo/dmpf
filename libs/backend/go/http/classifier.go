// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (RST-02) ou FND-08 (TRC-12, MET-07) que o símbolo realiza, dentro do limite de 3 linhas.

package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"
)

// RetryableStatusError is a response whose status the route declared
// transient, turned into an error so the retry conjunction can decide on it;
// the response is kept, so the last one is what the caller gets back.
type RetryableStatusError struct {
	Status   int
	Response *http.Response
}

func (e *RetryableStatusError) Error() string {
	return fmt.Sprintf("http: retryable status %d", e.Status)
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

func categoryOf(err error) string {
	if err == nil {
		return observe.CategoryOK
	}
	var categorized interface{ ErrorCategory() string }
	if errors.As(err, &categorized) {
		return categorized.ErrorCategory()
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "DeadlineExceeded"
	case errors.Is(err, context.Canceled):
		return "Cancelled"
	}
	var transient *RetryableStatusError
	if errors.As(err, &transient) {
		return statusCategory(transient.Status)
	}
	var network net.Error
	if errors.As(err, &network) {
		return categoryTransientDependency
	}
	return semconv.ErrorTypeOther.Value.AsString()
}

const categoryTransientDependency = "TransientDependency"

var fnd07Statuses = map[int]string{
	http.StatusBadRequest:          "Validation",
	http.StatusUnauthorized:        "Unauthenticated",
	http.StatusForbidden:           "Forbidden",
	http.StatusNotFound:            "NotFound",
	http.StatusConflict:            "Conflict",
	http.StatusUnprocessableEntity: "DomainRejection",
	http.StatusTooManyRequests:     "RateLimited",
}

func statusCategory(status int) string {
	if category, mapped := fnd07Statuses[status]; mapped {
		return category
	}
	if status >= 500 && status <= 599 {
		return categoryTransientDependency
	}
	return semconv.ErrorTypeOther.Value.AsString()
}
