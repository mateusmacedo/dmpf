// comment-discipline-ok-file: arquivo de declarações; o godoc registra que o passo 1 é gancho e que a taxonomia do erro é de FND-07, dentro do limite de 3 linhas.

package application

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// AuthorizeWithContext is step 1 of FND-04 §3.2 as a hook: the use case invokes
// it first and a returned error stops the sequence before step 2. The execution
// context is an explicit argument (CTX-03), and the decision reads permissions
// already resolved, reaching no authority of its own (IDN-10). The taxonomy of
// the authorization error belongs to FND-07 and is not modelled here.
type AuthorizeWithContext[C any] func(ctx context.Context, execution ports.ExecutionContext, cmd C) error

// AllowAllWithContext authorizes every operation whatever the context carries.
// It stays the composition root's explicit choice, never a silent default.
func AllowAllWithContext[C any]() AuthorizeWithContext[C] {
	return func(context.Context, ports.ExecutionContext, C) error { return nil }
}
