// comment-discipline-ok-file: godoc da fronteira de UoW; as cinco cláusulas de Within são a norma FND-04 §3.1-§3.3 enunciada como contrato, e o teste de contrato as executa.

package dmpfports

import "context"

// UnitOfWork is the application boundary of FND-04 §3.1, generic over the
// resource set R the use case declares.
//
// R is the use case's own type because UOW-03 requires the transactional ports
// to arrive as a typed argument and UOW-04 requires that only the bound ports
// arrive. A provider cannot know R — provider → application is a forbidden cell
// — so whoever knows both sides builds R from the open transaction: the
// composition root. Passing resources through context.Context is forbidden by
// CTX-05, and a tx.Repository("name") lookup inside the boundary is the service
// locator that UOW-03 rules out by name.
type UnitOfWork[R any] interface {
	// Within opens exactly one local transaction over one resource (UOW-01,
	// UOW-02), and every realization honours the same five clauses:
	//
	//   - if ctx.Err() is non-nil before the transaction opens, it returns that
	//     error and never invokes fn — CTX-21 read by analogy, from the remote
	//     I/O provider to the local transaction;
	//   - it invokes fn exactly once and never repeats it, under any error
	//     (UOW-09, UOW-10);
	//   - fn returning nil commits, and a commit error is returned as the
	//     provider produced it;
	//   - fn returning an error rolls back and returns that same error, without
	//     wrapping that would break errors.Is;
	//   - a panic in fn rolls back and propagates, never swallowed and never
	//     converted into a rejection (ERR-22).
	Within(ctx context.Context, fn func(ctx context.Context, resources R) error) error
}
