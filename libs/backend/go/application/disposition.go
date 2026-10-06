// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-04 §6.3-§6.4, INB-09/INB-11), dentro do limite de 3 linhas.

package application

import (
	"context"
	"errors"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Disposition is one of FND-04 §6.4's seven consumption outcomes: axis 1
// (R2, R3, R4) short-circuits axis 2, which exists only under R1 (INB-11).
type Disposition uint8

const (
	R1D1 Disposition = iota + 1
	R1D2
	R1D3
	R1D4
	R2
	R3
	R4
)

// String renders the disposition in the notation §6.4 itself uses.
func (d Disposition) String() string {
	switch d {
	case R1D1:
		return "R1×D1"
	case R1D2:
		return "R1×D2"
	case R1D3:
		return "R1×D3"
	case R1D4:
		return "R1×D4"
	case R2:
		return "R2"
	case R3:
		return "R3"
	case R4:
		return "R4"
	default:
		return "invalid"
	}
}

// Classify resolves a first reception's technical failure to D3 or D4
// (INB-09). A nil error is a programming defect: success and domain rejection
// reach the caller through the UPR's own outcome, never through Classify.
func Classify(err error) Disposition {
	if err == nil {
		panic("application: Classify requires a non-nil error")
	}

	var failure *Failure
	switch {
	case errors.As(err, &failure):
		if failure.Retryable() {
			return R1D3
		}
		return R1D4
	case errors.Is(err, ports.ErrRegisterTimeout):
		// INB-17: the wait for a concurrent key is a transient condition.
		return R1D3
	case errors.Is(err, context.DeadlineExceeded), transientDatabase(err), transientTransport(err):
		// ERR-10: under the inbox the interrupted effect is either rolled back or
		// committed and read as R2 on redelivery, the predicate a retry needs.
		return R1D3
	case errors.Is(err, context.Canceled):
		// CTX-23 + ERR-11: Cancelled has no declared predicate (FND-07 §5).
		return R1D4
	default:
		return R1D4
	}
}

// transientDatabase recognises the SQLSTATEs after which the transaction is gone
// and nothing was kept: connection exceptions (class 08), serialization failure,
// deadlock and administrator shutdown. The interface keeps the driver out.
func transientDatabase(err error) bool {
	var coded interface{ SQLState() string }
	if !errors.As(err, &coded) {
		return false
	}
	switch state := coded.SQLState(); {
	case strings.HasPrefix(state, "08"):
		return true
	default:
		return state == "40001" || state == "40P01" || state == "57P01"
	}
}

// transientTransport recognises a driver's own verdict: a failure before the
// statement reached the server (pgconn's SafeToRetry) or a network timeout.
func transientTransport(err error) bool {
	var unsent interface{ SafeToRetry() bool }
	if errors.As(err, &unsent) && unsent.SafeToRetry() {
		return true
	}
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}
