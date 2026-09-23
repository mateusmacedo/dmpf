// comment-discipline-ok-file: arquivo de declarações; o godoc registra que o passo 1 é gancho e que a taxonomia do erro é de FND-07, dentro do limite de 3 linhas.

package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Authorize is step 1 of FND-04 §3.2 as a hook: the use case invokes it first
// and a returned error stops the sequence before step 2. The execution context
// comes off the carrier (CTX-03, ADR-049), and the decision reads permissions
// already resolved, reaching no authority of its own (IDN-10). The taxonomy of
// the authorization error belongs to FND-07 and is not modelled here.
type Authorize[C any] func(ctx context.Context, cmd C) error

// AllowAll authorizes every operation whatever the carrier holds. It stays the
// composition root's explicit choice, never a silent default.
func AllowAll[C any]() Authorize[C] {
	return func(context.Context, C) error { return nil }
}

// Permitted decides step 1 by the permission each operation declares (IDN-16)
// over the permissions already on the carrier (IDN-10). Tenant and permission
// are separate checks (IDN-08), and an undeclared operation is denied (IDN-17).
//
// WHY: a chain without a subject passes on its tenant alone, because IDN-18
// authorizes it by the workload that runs it, whose identity the channel
// verified before the context existed (IDN-03, CTX-27).
func Permitted[C any](required func(C) ports.Permission) Authorize[C] {
	return func(ctx context.Context, cmd C) error {
		execution, err := ports.RequireExecutionContext(ctx)
		if err != nil {
			return err
		}
		if _, scoped := execution.Tenant(); !scoped {
			return fmt.Errorf("%w: the operation reaches tenant data and no tenant was resolved", ports.ErrDenied)
		}
		permission := required(cmd)
		if permission == "" {
			return fmt.Errorf("%w: the operation declares no permission", ports.ErrDenied)
		}
		if _, identified := execution.Subject(); !identified {
			return nil
		}
		if !slices.Contains(execution.Permissions(), permission) {
			return fmt.Errorf("%w: the subject does not hold %s", ports.ErrDenied, permission)
		}
		return nil
	}
}
