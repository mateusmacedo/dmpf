// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (§6.3, INB-08), dentro do limite de 3 linhas.

package ports

import "context"

// Acknowledger is the broker effect of a disposition, applied only after the
// unit of work commits or rolls back (INB-08).
type Acknowledger interface {
	// Ack confirms the message: D1, D2, R2 and R3 all reach it (§6.4).
	Ack(ctx context.Context) error

	// Release does not confirm; the transport redelivers with backoff, a
	// concrete gesture left to KRN-10 (D3).
	Release(ctx context.Context) error
}
