// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-04 §3.3, UOW-09/11, IDN-12/13), dentro do limite de 3 linhas.

package ports

import (
	"context"
	"errors"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// ErrNotFound is what Load reports when the aggregate does not exist. A
// provider may wrap it; errors.Is recovers it through any wrapping.
var ErrNotFound = errors.New("ports: aggregate not found")

// CrossTenantAccess is how a Reader reports that the identifier exists for
// another tenant. It unwraps to ErrNotFound and reads the same, so the answer
// stays indistinguishable (IDN-13) while the internal record tells (IDN-12).
type CrossTenantAccess struct {
	Object        string
	ContextTenant TenantID
	DataTenant    TenantID
}

func (CrossTenantAccess) Error() string { return ErrNotFound.Error() }

func (CrossTenantAccess) Unwrap() error { return ErrNotFound }

// ErrVersionConflict is what Save reports when the stored version differs from
// expected. Within never retries the callback on it (UOW-09, UOW-10): the retry
// policy, when it exists, belongs to the use case.
var ErrVersionConflict = errors.New("ports: version conflict")

// ErrAlreadyExists is what a use case reports when the identity it was told to
// create is taken. Save keeps reporting ErrVersionConflict for that case, so the
// use case that creates without loading is the one that tells them apart.
var ErrAlreadyExists = errors.New("ports: aggregate already exists")

// ErrUniqueViolation is what Save reports, through UniqueViolation, when a write
// breaks a unique constraint that is not the identity.
var ErrUniqueViolation = errors.New("ports: unique constraint violated")

// UniqueViolation carries the constraint Save broke and the rejection the context
// declared for it; a nil Rejection means the violation is not convertible, either
// because the constraint is undeclared or because the transaction already wrote.
type UniqueViolation struct {
	Table      string
	Constraint string
	Rejection  *domain.Rejection
}

func (UniqueViolation) Error() string { return ErrUniqueViolation.Error() }

func (UniqueViolation) Unwrap() error { return ErrUniqueViolation }

// Reader loads persisted aggregate state. It exists apart from Repository so a
// query can be given read access without the write side (UOW-11).
type Reader[ID comparable, S any] interface {
	Load(ctx context.Context, id ID) (S, Version, error)
}

// Repository adds optimistic-locking writes to Reader (FND-04 §3.3), typed over
// the persisted state S rather than the aggregate: a UPR mutates its receiver on
// acceptance, and a shared pointer would leak that before the commit.
type Repository[ID comparable, S any] interface {
	Reader[ID, S]

	// Save writes state as version expected+1 if, and only if, the stored version
	// equals expected (0 creates); divergence is ErrVersionConflict. Neither it nor
	// a declared UniqueViolation writes anything, and the transaction stays usable.
	Save(ctx context.Context, id ID, state S, expected Version) error
}
