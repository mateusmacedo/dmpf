package logging_test

import (
	"testing"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func TestTheLogKeysAreTheSharedVocabulary(t *testing.T) {
	for name, pair := range map[string][2]string{
		"KeyCorrelationID":   {logging.KeyCorrelationID, tracing.KeyCorrelationID},
		"KeyRequestID":       {logging.KeyRequestID, tracing.KeyRequestID},
		"KeyTenantID":        {logging.KeyTenantID, tracing.KeyTenantID},
		"KeyOutcomeCategory": {logging.KeyOutcomeCategory, tracing.KeyOutcomeCategory},
		"KeyAttempt":         {logging.KeyAttempt, tracing.KeyAttempt},
		"KeyDisposition":     {logging.KeyDisposition, tracing.KeyInboxDisposition},
		"KeyGesture":         {logging.KeyGesture, tracing.KeyInboxGesture},
		"KeyIdempotencyKey":  {logging.KeyIdempotencyKey, tracing.KeyIdempotencyKey},
		"KeyStatus":          {logging.KeyStatus, string(semconv.HTTPResponseStatusCodeKey)},
		"KeyCode":            {logging.KeyCode, string(semconv.RPCResponseStatusCodeKey)},
		"KeyChannel":         {logging.KeyChannel, string(semconv.MessagingDestinationNameKey)},
		"KeyMessageID":       {logging.KeyMessageID, string(semconv.MessagingMessageIDKey)},
	} {
		if pair[0] != pair[1] {
			t.Errorf("logging.%s = %q, want %q from the shared vocabulary (RF-A3)", name, pair[0], pair[1])
		}
	}
}

func TestErrorKeysAreTheOnesRedactWrites(t *testing.T) {
	if logging.KeyErrorType != redact.KeyErrorType {
		t.Errorf("KeyErrorType = %q, want redact.KeyErrorType %q", logging.KeyErrorType, redact.KeyErrorType)
	}
	if logging.KeyErrorCode != redact.KeyErrorCode {
		t.Errorf("KeyErrorCode = %q, want redact.KeyErrorCode %q", logging.KeyErrorCode, redact.KeyErrorCode)
	}
}
