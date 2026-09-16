// comment-discipline-ok-file: arquivo de declarações; o godoc registra que o passo 1 é gancho e que a taxonomia do erro é de FND-07, dentro do limite de 3 linhas.

package application

import "context"

// AuthorizeFunc is step 1 of FND-04 §3.2 as a hook: the use case invokes it
// first and a returned error stops the sequence before step 2. The taxonomy of
// the authorization error belongs to FND-07 and is not modelled here.
type AuthorizeFunc[C any] func(ctx context.Context, cmd C) error

// AllowAll authorizes every command. It is the composition root's explicit
// choice for a kernel that has no authorization yet, never a silent default.
func AllowAll[C any]() AuthorizeFunc[C] {
	return func(context.Context, C) error { return nil }
}
