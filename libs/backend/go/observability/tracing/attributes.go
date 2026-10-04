package tracing

import (
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Keys of the permitted span attributes (TRC-04). No key exists for an error, a
// payload or a domain type: a span that carried them would leave the process
// with content redaction was meant to remove (TRC-15).
const (
	KeyCorrelationID   = "dmpf.correlation_id"
	KeyRequestID       = "dmpf.request_id"
	KeyTenantID        = "dmpf.tenant_id"
	KeyOutcomeCategory = "dmpf.outcome_category"
	KeyTrafficClass    = "dmpf.traffic_class"
	KeyDependency      = "dmpf.dependency"
	KeyOperation       = "dmpf.operation"
	KeyAttempt         = "dmpf.retry.attempt"

	KeyOutboxClaimID = "dmpf.outbox.claim_id"
	KeyOutboxAttempt = "dmpf.outbox.attempt"

	KeyErrorCode             = "dmpf.error.code"
	KeyInboxAttempt          = "dmpf.inbox.attempt"
	KeyInboxDisposition      = "dmpf.inbox.disposition"
	KeyInboxGesture          = "dmpf.inbox.gesture"
	KeyIdempotencyKey        = "dmpf.idempotency_key"
	KeyIdempotencyKeyDerived = "dmpf.idempotency_key.derived"
	KeyIdempotencyKeyInvalid = "dmpf.idempotency_key.invalid"
	KeyIdempotencyOutcome    = "dmpf.idempotency_outcome"
	KeyDeadlineRemainingMS   = "dmpf.deadline.remaining_ms"
	KeyProcessRole           = "dmpf.process.role"
)

// Attributes is a closed builder: one method per permitted key, none of them
// taking an error, a payload or a domain type. The methods return a new value,
// so a partially built set is safe to share.
type Attributes struct {
	kv []attribute.KeyValue
}

func (a Attributes) with(kv attribute.KeyValue) Attributes {
	next := make([]attribute.KeyValue, len(a.kv), len(a.kv)+1)
	copy(next, a.kv)
	return Attributes{kv: append(next, kv)}
}

// withString omits an empty value: absence is information, and a blank
// attribute would read as a value that is not there (CTX-26).
func (a Attributes) withString(key, value string) Attributes {
	if value == "" {
		return a
	}
	return a.with(attribute.String(key, value))
}

// CorrelationID is the identifier that ties the whole flow together.
func (a Attributes) CorrelationID(value string) Attributes {
	return a.withString(KeyCorrelationID, value)
}

// RequestID is the identifier of this request.
func (a Attributes) RequestID(value string) Attributes { return a.withString(KeyRequestID, value) }

// TenantID is the tenant, omitted when absent.
func (a Attributes) TenantID(value string) Attributes { return a.withString(KeyTenantID, value) }

// OutcomeCategory is the terminal category of the operation.
func (a Attributes) OutcomeCategory(value string) Attributes {
	return a.withString(KeyOutcomeCategory, value)
}

// TrafficClass separates read from write traffic, and drives the sampling rate.
func (a Attributes) TrafficClass(value string) Attributes {
	return a.withString(KeyTrafficClass, value)
}

// Dependency names the dependency the span is about.
func (a Attributes) Dependency(value string) Attributes { return a.withString(KeyDependency, value) }

// Attempt is the attempt number, counted from 1, as the retry event and the
// transport log count it (TRC-11).
func (a Attributes) Attempt(value int) Attributes {
	return a.with(attribute.Int(KeyAttempt, value))
}

func (a Attributes) OutboxClaimID(value string) Attributes {
	return a.withString(KeyOutboxClaimID, value)
}

// OutboxAttempt counts the claims of a record, as outbox.attempt_count does
// (postgres/claim.go:63), and not the calls of one retry, as Attempt does.
func (a Attributes) OutboxAttempt(value int) Attributes {
	return a.with(attribute.Int(KeyOutboxAttempt, value))
}

func (a Attributes) MessagingDestinationPartitionID(value string) Attributes {
	if value == "" {
		return a
	}
	return a.with(semconv.MessagingDestinationPartitionID(value))
}

func (a Attributes) MessagingKafkaOffset(value int) Attributes {
	return a.with(semconv.MessagingKafkaOffset(value))
}

func ExecutionAttributes(execution ports.ExecutionContext) Attributes {
	attributes := Attributes{}.
		CorrelationID(execution.CorrelationID()).
		RequestID(execution.RequestID())
	if tenant, scoped := execution.Tenant(); scoped {
		attributes = attributes.TenantID(string(tenant))
	}
	return attributes
}

// KeyValues is the set to pass to a span. It returns a copy, so a recorded set
// cannot be rewritten through the slice it was built from.
func (a Attributes) KeyValues() []attribute.KeyValue {
	return append([]attribute.KeyValue(nil), a.kv...)
}
