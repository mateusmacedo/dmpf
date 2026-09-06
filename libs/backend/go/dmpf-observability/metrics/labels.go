package metrics

import "go.opentelemetry.io/otel/attribute"

// Keys of the permitted labels (MET-04). There is deliberately no key for an
// identifier, a message, a correlation, a request, a user or a tenant: those
// are unbounded or personal, and a series that carried them would either
// explode in cardinality or leak (MET-07).
const (
	KeyDependency      = "dependency"
	KeyOperation       = "operation"
	KeyService         = "service"
	KeyErrorCategory   = "error_category"
	KeyOutcomeCategory = "outcome_category"
)

// Labels is a closed builder: one method per permitted key and no generic
// setter, so a forbidden label cannot reach a series by accident. The methods
// return a new value, so a partially built set is safe to share.
type Labels struct {
	kv []attribute.KeyValue
}

func (l Labels) with(key, value string) Labels {
	if value == "" {
		return l
	}

	next := make([]attribute.KeyValue, len(l.kv), len(l.kv)+1)
	copy(next, l.kv)
	return Labels{kv: append(next, attribute.String(key, value))}
}

// Dependency names the dependency the measurement is about.
func (l Labels) Dependency(value string) Labels { return l.with(KeyDependency, value) }

// Operation names the operation the measurement is about.
func (l Labels) Operation(value string) Labels { return l.with(KeyOperation, value) }

// Service names the service the measurement is about.
func (l Labels) Service(value string) Labels { return l.with(KeyService, value) }

// ErrorCategory is the category of the failure, never its message.
func (l Labels) ErrorCategory(value string) Labels { return l.with(KeyErrorCategory, value) }

// OutcomeCategory is the terminal category of the operation.
func (l Labels) OutcomeCategory(value string) Labels { return l.with(KeyOutcomeCategory, value) }

// Attributes is the set to pass to an instrument. It returns a copy, so a
// recorded measurement cannot be rewritten through the slice it was built from.
func (l Labels) Attributes() []attribute.KeyValue {
	return append([]attribute.KeyValue(nil), l.kv...)
}
