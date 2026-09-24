// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-08 (TRC-12, MET-07) que o símbolo realiza, dentro do limite de 3 linhas.

package sqs

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/aws/smithy-go"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"
)

// Classifier is the retry taxonomy of the SDK calls: a throttle or a server
// fault and a network failure are worth another attempt; a cancellation, a
// deadline and any other API error are not.
func Classifier(err error) retry.Retryability {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return retry.NotRetryable
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		switch api.ErrorFault() {
		case smithy.FaultServer:
			return retry.Retryable
		}
		if isThrottle(api.ErrorCode()) {
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

func isThrottle(code string) bool {
	switch code {
	case "Throttling", "ThrottlingException", "RequestThrottled", "RequestThrottledException",
		"TooManyRequestsException", "RequestLimitExceeded", "ServiceUnavailable", "InternalError":
		return true
	}
	return false
}

// categoryOf returns the bounded category under which a failure is recorded:
// a platform category, the API error code in lowercase, the context error or
// "network" — never the message (TRC-12, MET-07).
func categoryOf(err error) string {
	if err == nil {
		return observe.CategoryOK
	}
	var categorized interface{ ErrorCategory() string }
	if errors.As(err, &categorized) {
		return categorized.ErrorCategory()
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		return strings.ToLower(api.ErrorCode())
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
