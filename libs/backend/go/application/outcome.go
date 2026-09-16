// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-03 §6, DEC-01..DEC-04, ADR-018), dentro do limite de 3 linhas.

package application

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// Outcome separates the business channel from the technical one: a use case
// returns (Outcome[R], error), where error carries only technical failure and
// the refusal travels here with error == nil (DEC-04, ADR-018).
type Outcome[R any] struct {
	response  R
	rejection *domain.Rejection
}

// Accepted builds the accepting branch of the outcome.
func Accepted[R any](response R) Outcome[R] {
	return Outcome[R]{response: response}
}

// Rejected builds the refusing branch. A nil rejection is a programming defect,
// because absence of rejection is not a third outcome (DEC-01), so it panics
// rather than producing an outcome that reads as accepted.
func Rejected[R any](rejection *domain.Rejection) Outcome[R] {
	if rejection == nil {
		panic("application: Rejected requires a rejection; use Accepted for the accepting branch")
	}
	return Outcome[R]{rejection: rejection}
}

// Response is the domain response, and is the zero value on the refusing branch.
func (o Outcome[R]) Response() R { return o.response }

// Rejection reports the refusal and whether there was one, so the caller
// exhausts both branches without inspecting error (DEC-01).
func (o Outcome[R]) Rejection() (*domain.Rejection, bool) {
	return o.rejection, o.rejection != nil
}
