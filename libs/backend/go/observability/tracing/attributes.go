package tracing

import "go.opentelemetry.io/otel/attribute"

// Keys of the permitted span attributes (TRC-04). No key exists for an error, a
// payload or a domain type: a span that carried them would leave the process
// with content redaction was meant to remove (TRC-15).
const (
	KeyCorrelationID   = "dmpf.correlation_id"
	KeyRequestID       = "dmpf.request_id"
	KeyTenantID        = "dmpf.tenant_id"
	KeyService         = "dmpf.service"
	KeyVersion         = "dmpf.version"
	KeyOutcomeCategory = "dmpf.outcome_category"
	KeyTrafficClass    = "dmpf.traffic_class"
	KeyDependency      = "dmpf.dependency"
	KeyOperation       = "dmpf.operation"
	KeyAttempt         = "dmpf.retry.attempt"
	KeyErrorCategory   = "dmpf.error.category"
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

// Service names the service that owns the span.
func (a Attributes) Service(value string) Attributes { return a.withString(KeyService, value) }

// Version is the build of the service.
func (a Attributes) Version(value string) Attributes { return a.withString(KeyVersion, value) }

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

// Operation names the operation the span is about.
func (a Attributes) Operation(value string) Attributes { return a.withString(KeyOperation, value) }

// Attempt is the attempt number. Zero is the original call and is recorded, so
// a reader distinguishes "first attempt" from "not instrumented".
func (a Attributes) Attempt(value int) Attributes {
	return a.with(attribute.Int(KeyAttempt, value))
}

// KeyValues is the set to pass to a span. It returns a copy, so a recorded set
// cannot be rewritten through the slice it was built from.
func (a Attributes) KeyValues() []attribute.KeyValue {
	return append([]attribute.KeyValue(nil), a.kv...)
}
