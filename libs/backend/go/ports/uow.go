// comment-discipline-ok-file: o godoc de Within é o contrato normativo de FND-04 §3.1-§3.3 enunciado como API pública, e é o que a suíte RunUnitOfWorkContract executa.

package ports

import "context"

// UnitOfWork is the application boundary of FND-04 §3.1, generic over the
// resource set R the use case declares: UOW-03 requires typed ports, UOW-04
// only the bound ones, so the composition root binds the transaction to R.
type UnitOfWork[R any] interface {
	// Within opens exactly one local transaction over one resource (UOW-01,
	// UOW-02), and every realization honours the same six clauses:
	//
	//   - if ctx.Err() is non-nil before the transaction opens, it returns that
	//     error and never invokes fn — CTX-21 read by analogy, from the remote
	//     I/O provider to the local transaction;
	//   - it invokes fn exactly once and never repeats it, under any error
	//     (UOW-09, UOW-10);
	//   - fn returning nil commits;
	//   - fn returning an error rolls back and returns that same error, without
	//     wrapping that would break errors.Is;
	//   - a commit error is returned as the provider produced it, and nothing
	//     persists;
	//   - a panic in fn rolls back and propagates, never swallowed and never
	//     converted into a rejection (ERR-22).
	//
	// R is built by the realization from the open transaction and is the only
	// path by which transactional ports reach fn: not context.Context (CTX-05),
	// not a tx.Repository("name") lookup, which UOW-03 rules out by name.
	Within(ctx context.Context, fn func(ctx context.Context, resources R) error) error
}
