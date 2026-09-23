// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-07 (IDN-06, IDN-15, CTX-03, CTX-05) que o símbolo realiza, dentro do limite de 3 linhas.

package http

import (
	"context"
	"net/http"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Resolved is what verification yielded, already shaped as the context spec
// takes it: absence stays absence, and no field is invented to satisfy a
// requirement (IDN-20).
type Resolved struct {
	Subject     *ports.SubjectID
	Tenant      *ports.TenantID
	Permissions []ports.Permission
}

// ResolveIdentity decides what the route demands against what the credential
// yielded. A zero status means the request proceeds; otherwise the caller
// writes the refusal in the shape its own contract publishes.
//
// WHY: the two refusals stay apart because IDN-06 forbids answering "not
// authenticated" to a subject that authenticated and merely lacks what the
// operation needs — it tells the caller the wrong thing about the wrong problem.
func ResolveIdentity(ctx context.Context, authenticator ports.Authenticator, route Route, credential ports.Credential) (Resolved, int, string) {
	if !credential.Presented() {
		if route.RequiresSubject() || route.RequiresTenant() {
			return Resolved{}, http.StatusUnauthorized, "unauthenticated"
		}
		return Resolved{}, 0, ""
	}

	identity, err := authenticator.Authenticate(ctx, credential)
	if err != nil {
		return Resolved{}, http.StatusUnauthorized, "unauthenticated"
	}
	if route.RequiresSubject() && identity.Subject == "" {
		return Resolved{}, http.StatusUnauthorized, "unauthenticated"
	}
	if route.RequiresTenant() && identity.Tenant == nil {
		return Resolved{}, http.StatusForbidden, "tenant-unresolved"
	}

	subject := identity.Subject
	return Resolved{Subject: &subject, Tenant: identity.Tenant, Permissions: identity.Permissions}, 0, ""
}

// WithExecutionContext deposits what the edge mounted on the canonical carrier.
// It delegates rather than keying its own value: a second key would make the
// provider read from a carrier the edge never wrote to (CTX-05, ADR-049).
func WithExecutionContext(ctx context.Context, execution ports.ExecutionContext) context.Context {
	return ports.WithExecutionContext(ctx, execution)
}

// ExecutionContextFrom returns what the edge deposited, off the same carrier
// every block downstream reads (CTX-03).
func ExecutionContextFrom(ctx context.Context) (ports.ExecutionContext, bool) {
	return ports.ExecutionContextFrom(ctx)
}
