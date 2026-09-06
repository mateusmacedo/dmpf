// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (RST-02, RST-03, RST-04) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfhttp

import (
	"errors"
	"fmt"
	"net/http"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/deadline"
)

var (
	// ErrContractRequired is a route that references no published contract:
	// a channel the provider cannot reference is not operable (RST-04).
	ErrContractRequired = errors.New("dmpfhttp: route declares no contract reference (RST-04)")

	// ErrMethodNotAllowed is a route whose method is outside the set the
	// provider knows how to classify for retry (RST-02).
	ErrMethodNotAllowed = errors.New("dmpfhttp: route method is not one of GET, HEAD, PUT, DELETE, POST, PATCH")

	// ErrIncompleteRoute is a route without name or path.
	ErrIncompleteRoute = errors.New("dmpfhttp: route is incomplete")
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
		return fmt.Errorf("dmpfhttp: %s: %w", r.Name, err)
	}
	return nil
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
