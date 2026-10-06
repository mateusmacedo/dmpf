package logging

import (
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

// Keys of the correlation fields of LOG-04.
const (
	KeyCorrelationID = tracing.KeyCorrelationID
	KeyRequestID     = tracing.KeyRequestID
	KeyTenantID      = tracing.KeyTenantID
)

const (
	KeyOutcomeCategory = tracing.KeyOutcomeCategory
	KeyErrorType       = redact.KeyErrorType
	KeyErrorCode       = redact.KeyErrorCode
	KeyStatus          = string(semconv.HTTPResponseStatusCodeKey)
	KeyCode            = string(semconv.RPCResponseStatusCodeKey)
	KeyChannel         = string(semconv.MessagingDestinationNameKey)
	KeyMessageID       = string(semconv.MessagingMessageIDKey)
	KeyAttempt         = tracing.KeyAttempt
	KeyDisposition     = tracing.KeyInboxDisposition
	KeyGesture         = tracing.KeyInboxGesture
	KeyIdempotencyKey  = tracing.KeyIdempotencyKey
	KeyBreakerState    = "dmpf.breaker.state"
)
