// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (§7.4, GAR-07/11/12), dentro do limite de 3 linhas.

package dmpfports

import "context"

// Reason names why a message was contained, stable text stored by the
// provider and grouped for signals (GAR-11, GAR-12).
type Reason string

const (
	ReasonInvalidEnvelope   Reason = "invalid-envelope"
	ReasonTerminalFailure   Reason = "terminal-failure"
	ReasonCollision         Reason = "collision"
	ReasonAttemptsExhausted Reason = "attempts-exhausted"
)

// Contained is what Containment.Quarantine persists: the raw envelope exactly
// as transported, never re-marshaled (GAR-07).
type Contained struct {
	Consumer  string
	MessageID MessageID
	Reason    Reason
	Envelope  []byte
	Error     string
	At        Instant
}

// Containment removes a message from normal flow outside any unit of work
// (GAR-07, GAR-11), with a sanitized error (ERR-20, ERR-21).
type Containment interface {
	Quarantine(ctx context.Context, c Contained) error
}
