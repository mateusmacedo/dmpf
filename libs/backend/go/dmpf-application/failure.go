// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-07 §5.2-§5.4, ERR-04, ERR-09), dentro do limite de 3 linhas.

package dmpfapplication

import "fmt"

// Category is one of FND-07 §5.3's eleven error categories; ERR-01/ERR-02
// make the catalog total, with Unexpected as the closing case.
type Category string

const (
	Validation          Category = "Validation"
	DomainRejection     Category = "DomainRejection"
	NotFound            Category = "NotFound"
	Conflict            Category = "Conflict"
	Forbidden           Category = "Forbidden"
	Unauthenticated     Category = "Unauthenticated"
	TransientDependency Category = "TransientDependency"
	RateLimited         Category = "RateLimited"
	DeadlineExceeded    Category = "DeadlineExceeded"
	Cancelled           Category = "Cancelled"
	Unexpected          Category = "Unexpected"
)

// Failure is a technical error already classified by FND-07 §5.2: retryability
// is resolved to a boolean at classification time (ERR-09, MAP-07), and
// Failure only transports the decision — it never recomputes it.
type Failure struct {
	category  Category
	retryable bool
	cause     error
}

// NewFailure builds a classified failure; cause may be nil.
func NewFailure(category Category, retryable bool, cause error) *Failure {
	return &Failure{category: category, retryable: retryable, cause: cause}
}

// Category reports the classification assigned where the failure was known (ERR-03).
func (f *Failure) Category() Category { return f.category }

// Retryable is the boolean FND-07 §5.4 requires every concrete error to resolve to.
func (f *Failure) Retryable() bool { return f.retryable }

// Error renders the category alone, or with the cause when one was given.
func (f *Failure) Error() string {
	if f.cause == nil {
		return string(f.category)
	}
	return fmt.Sprintf("%s: %s", f.category, f.cause)
}

// Unwrap exposes the cause to errors.Is and errors.As.
func (f *Failure) Unwrap() error { return f.cause }
