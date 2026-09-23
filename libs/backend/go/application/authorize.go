// comment-discipline-ok-file: arquivo de declarações; o godoc registra que o passo 1 é gancho e que a taxonomia do erro é de FND-07, dentro do limite de 3 linhas.

package application

import "context"

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
