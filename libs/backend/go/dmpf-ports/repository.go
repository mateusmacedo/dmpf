// comment-discipline-ok-file: godoc de portas; o contrato de optimistic locking e a separação Reader/Repository vêm de FND-04 §3.3 e UOW-11, não do código.

package dmpfports

import (
	"context"
	"errors"
)

// ErrNotFound is what Load reports when the aggregate does not exist. A
// provider may wrap it; errors.Is recovers it through any wrapping.
var ErrNotFound = errors.New("dmpfports: aggregate not found")

// ErrVersionConflict is what Save reports when the stored version differs from
// expected. Within never retries the callback on it (UOW-09, UOW-10): the retry
// policy, when it exists, belongs to the use case.
var ErrVersionConflict = errors.New("dmpfports: version conflict")

// Reader loads persisted aggregate state. It exists apart from Repository so a
// query can be given read access without the write side (UOW-11).
type Reader[ID comparable, S any] interface {
	Load(ctx context.Context, id ID) (S, Version, error)
}

// Repository adds optimistic-locking writes to Reader (FND-04 §3.3). It is
// typed over the persisted state S, not over the aggregate, because a UPR
// mutates its receiver on acceptance: sharing a pointer with the store would
// leak that mutation before the commit and void the atomicity proof.
type Repository[ID comparable, S any] interface {
	Reader[ID, S]

	// Save writes state as version expected+1 if, and only if, the stored
	// version equals expected; expected == 0 creates the aggregate. Any
	// divergence returns ErrVersionConflict and writes nothing.
	Save(ctx context.Context, id ID, state S, expected Version) error
}
