// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (RST-02, RST-03, RST-04) que o símbolo realiza, dentro do limite de 3 linhas.

package http

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

var (
	// ErrContractRequired is a route that references no published contract:
	// a channel the provider cannot reference is not operable (RST-04).
	ErrContractRequired = errors.New("http: route declares no contract reference (RST-04)")

	// ErrMethodNotAllowed is a route whose method is outside the set the
	// provider knows how to classify for retry (RST-02).
	ErrMethodNotAllowed = errors.New("http: route method is not one of GET, HEAD, PUT, DELETE, POST, PATCH")

	// ErrIncompleteRoute is a route without name or path.
	ErrIncompleteRoute = errors.New("http: route is incomplete")

	// ErrRequirementUnknown is a route whose declaration is outside the four
	// cases of IDN-16, which the provider cannot evaluate as a predicate.
	ErrRequirementUnknown = errors.New("http: route requirement is not one of the four cases of IDN-16")

	// ErrPlatformReachRequired is a platform route that leaves its data reach
	// undeclared, or a scoped route that declares one: reach is declared by the
	// platform operation alone, never obtained as a side effect (IDN-19).
	ErrPlatformReachRequired = errors.New("http: platform route must declare its data reach, and only it may (IDN-19)")

	// ErrPermissionRequired is an inbound route that demands a subject and
	// declares no permission: the edge would deny every call it serves (IDN-17).
	ErrPermissionRequired = errors.New("http: inbound route demanding a subject declares no permission (IDN-16)")
)

// Requirement is what an operation declares it needs resolved in the execution
// context (IDN-16). The zero value demands subject and tenant, so omitting the
// declaration closes the route instead of opening it (IDN-17).
type Requirement uint8

const (
	RequireSubjectAndTenant Requirement = iota
	RequireSubject
	RequireTenant
	RequireNeither
)

// PlatformReach is the data reach a platform operation declares (IDN-19).
// Reaching every tenant is a registered decision, never the side effect of a
// context carrying no tenant.
type PlatformReach uint8

const (
	ReachUndeclared PlatformReach = iota
	ReachAllTenants
)

var allowedMethods = map[string]struct{}{
	http.MethodGet: {}, http.MethodHead: {}, http.MethodPut: {}, http.MethodDelete: {},
	http.MethodPost: {}, http.MethodPatch: {},
}

// Route is one outbound HTTP route: the contract it references (RST-04), the
// deadline budget its timeout derives from (RST-03), the statuses that count
// as transient and, for POST, the header that carries the idempotency key.
type Route struct {
	Name            string
	Method          string
	Path            string
	ContractRef     string
	Budget          deadline.Budget
	RetryableStatus []int
	IdempotencyKey  string
	Requires        Requirement
	PlatformReach   PlatformReach
	Permission      ports.Permission
}

// Validate refuses a route without name, path, contract or a known method,
// and one whose budget the deadline package refuses.
func (r Route) Validate() error {
	switch {
	case r.Name == "":
		return fmt.Errorf("%w: name", ErrIncompleteRoute)
	case r.Path == "":
		return fmt.Errorf("%w: %s: path", ErrIncompleteRoute, r.Name)
	case r.ContractRef == "":
		return fmt.Errorf("%w: %s", ErrContractRequired, r.Name)
	}
	if _, allowed := allowedMethods[r.Method]; !allowed {
		return fmt.Errorf("%w: %s: %q", ErrMethodNotAllowed, r.Name, r.Method)
	}
	if err := r.Budget.Validate(); err != nil {
		return fmt.Errorf("http: %s: %w", r.Name, err)
	}
	if r.Requires > RequireNeither {
		return fmt.Errorf("%w: %s: %d", ErrRequirementUnknown, r.Name, r.Requires)
	}
	if platform := r.Requires == RequireNeither; platform != (r.PlatformReach == ReachAllTenants) {
		return fmt.Errorf("%w: %s", ErrPlatformReachRequired, r.Name)
	}
	return nil
}

// ValidateEdge is Validate for an inbound route, which also declares the
// permission its subject needs (IDN-16).
func (r Route) ValidateEdge() error {
	if err := r.Validate(); err != nil {
		return err
	}
	if r.RequiresSubject() && r.Permission == "" {
		return fmt.Errorf("%w: %s", ErrPermissionRequired, r.Name)
	}
	return nil
}

// RequiresSubject reports whether the operation demands an authenticated
// subject in the context; a route that declares nothing does (IDN-17).
func (r Route) RequiresSubject() bool {
	return r.Requires == RequireSubjectAndTenant || r.Requires == RequireSubject
}

// RequiresTenant reports whether the operation demands a resolved tenant in
// the context; a route that declares nothing does (IDN-17).
func (r Route) RequiresTenant() bool {
	return r.Requires == RequireSubjectAndTenant || r.Requires == RequireTenant
}

// Idempotent is what RST-02 admits a retry for: GET, HEAD, PUT and DELETE by
// their semantics, POST only when the route declares an idempotency key the
// provider sends, and PATCH never.
func (r Route) Idempotent() bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete:
		return true
	case http.MethodPost:
		return r.IdempotencyKey != ""
	default:
		return false
	}
}
