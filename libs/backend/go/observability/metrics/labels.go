package metrics

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

// Keys of the permitted labels (MET-04, RF-D3): semconv or dmpf.*. No key for
// an identifier, message, correlation, request or user (MET-07); the tenant
// enters only through TenantWithin, a declared set.
const (
	KeyDependency      = tracing.KeyDependency
	KeyOperation       = tracing.KeyOperation
	KeyErrorType       = "error.type"
	KeyOutcomeCategory = tracing.KeyOutcomeCategory
	KeyRoute           = "http.route"
	KeyRPCMethod       = "rpc.method"
	KeyTenant          = tracing.KeyTenantID
)

// KeyErrorCategory keeps the callers of the earlier vocabulary compiling.
const (
	KeyErrorCategory = KeyErrorType
)

// OtherTenant is the value every tenant outside the declared set collapses
// into, so the series stays bounded whatever the traffic brings (MET-07).
const OtherTenant = "other"

// Tenants is the declared, finite set of tenants a series may carry as a label
// (MET-07, MET-12). The zero value declares none: every tenant is "other".
type Tenants struct {
	set map[string]struct{}
}

// DeclareTenants fixes the set. It refuses an empty declaration and an empty
// name, because a label that admits any tenant is the cardinality MET-07 vetoes.
func DeclareTenants(names ...string) (Tenants, error) {
	if len(names) == 0 {
		return Tenants{}, fmt.Errorf("metrics: a tenant allowlist must declare at least one tenant (MET-07)")
	}
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			return Tenants{}, fmt.Errorf("metrics: a tenant allowlist must not carry an empty name")
		}
		set[name] = struct{}{}
	}
	return Tenants{set: set}, nil
}

// Resolve returns the tenant when it is declared, and OtherTenant otherwise.
func (t Tenants) Resolve(tenant string) string {
	if _, declared := t.set[tenant]; declared {
		return tenant
	}
	return OtherTenant
}

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

func (l Labels) ErrorType(value string) Labels { return l.with(KeyErrorType, value) }

func (l Labels) ErrorCategory(value string) Labels { return l.ErrorType(value) }

// OutcomeCategory is the terminal category of the operation.
func (l Labels) OutcomeCategory(value string) Labels { return l.with(KeyOutcomeCategory, value) }

func (l Labels) Route(value string) Labels { return l.with(KeyRoute, value) }

func (l Labels) RPCMethod(value string) Labels { return l.with(KeyRPCMethod, value) }

// TenantWithin is the only way a tenant reaches a series: resolved against the
// declared set, so anything outside it is recorded as OtherTenant (MET-07,
// MET-12). It is never omitted — a rejection with no tenant is still "other".
func (l Labels) TenantWithin(allowlist Tenants, tenant string) Labels {
	return l.with(KeyTenant, allowlist.Resolve(tenant))
}

// Attributes is the set to pass to an instrument. It returns a copy, so a
// recorded measurement cannot be rewritten through the slice it was built from.
func (l Labels) Attributes() []attribute.KeyValue {
	return append([]attribute.KeyValue(nil), l.kv...)
}
