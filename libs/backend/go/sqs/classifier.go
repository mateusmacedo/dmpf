// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-08 (TRC-12, MET-07) que o símbolo realiza, dentro do limite de 3 linhas.

package sqs

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"github.com/aws/smithy-go"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
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
		"TooManyRequestsException", "RequestLimitExceeded", "Throttled", "KmsThrottled", "KMSThrottling",
		"KMS.ThrottlingException", "ServiceUnavailable", "InternalError":
		return true
	}
	return false
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
	var api smithy.APIError
	if errors.As(err, &api) {
		return apiCategory(api)
	}
	var network net.Error
	if errors.As(err, &network) {
		return categoryTransientDependency
	}
	return semconv.ErrorTypeOther.Value.AsString()
}

func errorAttr(err error) slog.Attr {
	var categorized redact.Categorized
	if err == nil || errors.As(err, &categorized) {
		return redact.Error(err)
	}
	return slog.String(redact.KeyErrorType, categoryOf(err))
}

const categoryTransientDependency = "TransientDependency"

var fnd07Codes = map[string]string{
	"ServiceUnavailable":                      categoryTransientDependency,
	"InternalError":                           categoryTransientDependency,
	"Throttling":                              "RateLimited",
	"ThrottlingException":                     "RateLimited",
	"RequestThrottled":                        "RateLimited",
	"RequestThrottledException":               "RateLimited",
	"TooManyRequestsException":                "RateLimited",
	"RequestLimitExceeded":                    "RateLimited",
	"Throttled":                               "RateLimited",
	"KmsThrottled":                            "RateLimited",
	"KMSThrottling":                           "RateLimited",
	"KMS.ThrottlingException":                 "RateLimited",
	"QueueDoesNotExist":                       "NotFound",
	"AWS.SimpleQueueService.NonExistentQueue": "NotFound",
	"NotFound":                                "NotFound",
	"AccessDeniedException":                   "Forbidden",
	"AuthorizationError":                      "Forbidden",
	"KmsAccessDenied":                         "Forbidden",
	"KMSAccessDenied":                         "Forbidden",
	"KMS.AccessDeniedException":               "Forbidden",
	"InvalidSecurity":                         "Unauthenticated",
	"InvalidMessageContents":                  "Validation",
	"InvalidAttributeName":                    "Validation",
	"InvalidAttributeValue":                   "Validation",
	"InvalidParameter":                        "Validation",
	"ParameterValueInvalid":                   "Validation",
	"ValidationException":                     "Validation",
}

func apiCategory(api smithy.APIError) string {
	if category, mapped := fnd07Codes[api.ErrorCode()]; mapped {
		return category
	}
	if api.ErrorFault() == smithy.FaultServer {
		return categoryTransientDependency
	}
	return semconv.ErrorTypeOther.Value.AsString()
}
