// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-07 §3.1), dentro do limite de 3 linhas.

package ports

import (
	"errors"
	"fmt"
)

// SubjectID is the subject resolved by this request's own authentication
// (FND-07 §3.2): header, query and body are never the source. The empty
// identifier is invalid.
type SubjectID string

// TenantID is the tenant resolved by authentication, under the same predicate
// as ENV-12 (FND-07 §3.1). A synthetic value standing for "no tenant" is
// forbidden (IDN-20).
type TenantID string

// Permission is one effective permission resolved for the subject before the
// transaction opens; deciding with it reaches no authority (IDN-10).
type Permission string

// ErrContextFieldMissing reports a mandatory field of CTX-01 left out. The
// context is a construction defect and the operation does not proceed.
var ErrContextFieldMissing = errors.New("ports: execution context field missing")

// ErrContextValueEmpty reports a conditional field present with an empty value.
// Absence has its own shape in the spec, so "" never stands for unresolved.
var ErrContextValueEmpty = errors.New("ports: execution context value empty")

// ErrContextPermissionsMismatch reports permissions without a subject, or a
// subject without them: CTX-01 ties one presence to the other.
var ErrContextPermissionsMismatch = errors.New("ports: execution context permissions do not match the subject")

// ExecutionContextSpec is what the edge gathers before construction. The four
// conditional fields carry absence in the type, so no block can write "" where
// nothing was resolved (CTX-01).
type ExecutionContextSpec struct {
	RequestID     string
	CorrelationID string
	CausationID   *string
	TraceContext  string
	Subject       *SubjectID
	Tenant        *TenantID
	Permissions   []Permission
	Deadline      Instant
	Locale        string
}

// ExecutionContext is the nine-field context of CTX-01. The port declares the
// type, the app mounts the instance at the edge, and the application service
// takes it as an explicit argument (CTX-02, CTX-03).
type ExecutionContext struct {
	requestID     string
	correlationID string
	causationID   string
	traceContext  string
	subject       SubjectID
	tenant        TenantID
	permissions   []Permission
	deadline      Instant
	locale        string

	hasCausation bool
	hasSubject   bool
	hasTenant    bool
}

// NewExecutionContext validates presence by field and returns a value no block
// downstream can alter (CTX-04). Every rejection is a construction defect, not
// a business outcome.
func NewExecutionContext(spec ExecutionContextSpec) (ExecutionContext, error) {
	if spec.RequestID == "" {
		return ExecutionContext{}, fmt.Errorf("%w: request_id", ErrContextFieldMissing)
	}
	if spec.CorrelationID == "" {
		return ExecutionContext{}, fmt.Errorf("%w: correlation_id", ErrContextFieldMissing)
	}
	if spec.TraceContext == "" {
		return ExecutionContext{}, fmt.Errorf("%w: trace_context", ErrContextFieldMissing)
	}
	if spec.Deadline <= 0 {
		return ExecutionContext{}, fmt.Errorf("%w: deadline", ErrContextFieldMissing)
	}
	if spec.Locale == "" {
		return ExecutionContext{}, fmt.Errorf("%w: locale", ErrContextFieldMissing)
	}

	if spec.CausationID != nil && *spec.CausationID == "" {
		return ExecutionContext{}, fmt.Errorf("%w: causation_id", ErrContextValueEmpty)
	}
	if spec.Subject != nil && *spec.Subject == "" {
		return ExecutionContext{}, fmt.Errorf("%w: authenticated_subject", ErrContextValueEmpty)
	}
	if spec.Tenant != nil && *spec.Tenant == "" {
		return ExecutionContext{}, fmt.Errorf("%w: tenant_id", ErrContextValueEmpty)
	}

	if spec.Subject != nil && spec.Permissions == nil {
		return ExecutionContext{}, fmt.Errorf("%w: subject resolved without a permission set", ErrContextPermissionsMismatch)
	}
	if spec.Subject == nil && spec.Permissions != nil {
		return ExecutionContext{}, fmt.Errorf("%w: permission set without a resolved subject", ErrContextPermissionsMismatch)
	}

	ec := ExecutionContext{
		requestID:     spec.RequestID,
		correlationID: spec.CorrelationID,
		traceContext:  spec.TraceContext,
		deadline:      spec.Deadline,
		locale:        spec.Locale,
	}
	if spec.CausationID != nil {
		ec.causationID, ec.hasCausation = *spec.CausationID, true
	}
	if spec.Subject != nil {
		ec.subject, ec.hasSubject = *spec.Subject, true
		ec.permissions = clonePermissions(spec.Permissions)
	}
	if spec.Tenant != nil {
		ec.tenant, ec.hasTenant = *spec.Tenant, true
	}
	return ec, nil
}

// RequestID identifies this execution, unique per received request.
func (ec ExecutionContext) RequestID() string { return ec.requestID }

// CorrelationID identifies the business flow crossing services.
func (ec ExecutionContext) CorrelationID() string { return ec.correlationID }

// CausationID identifies the immediately preceding step; ok is false when no
// identifiable causing step exists.
func (ec ExecutionContext) CausationID() (string, bool) { return ec.causationID, ec.hasCausation }

// TraceContext is the distributed propagation context received or started at
// this edge.
func (ec ExecutionContext) TraceContext() string { return ec.traceContext }

// Subject is the authenticated subject; ok is false when the operation does not
// require identity, which §4 decides, never convenience.
func (ec ExecutionContext) Subject() (SubjectID, bool) { return ec.subject, ec.hasSubject }

// Tenant is the resolved tenant; ok is false only along the chain §4 declares
// legitimate.
func (ec ExecutionContext) Tenant() (TenantID, bool) { return ec.tenant, ec.hasTenant }

// Permissions returns a copy of the effective set, nil when no subject was
// resolved. The empty set is present and distinct from absence.
func (ec ExecutionContext) Permissions() []Permission {
	if !ec.hasSubject {
		return nil
	}
	return clonePermissions(ec.permissions)
}

// Deadline is the absolute instant past which the operation must not proceed.
func (ec ExecutionContext) Deadline() Instant { return ec.deadline }

// Locale is the language and formatting convention applicable to the response.
func (ec ExecutionContext) Locale() string { return ec.locale }

func clonePermissions(in []Permission) []Permission {
	out := make([]Permission, len(in))
	copy(out, in)
	return out
}
