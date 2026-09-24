// comment-discipline-ok-file: arquivo de declarações; o mapeamento situação → mecanismo é norma revisável em PR (GAR-11), no molde de conformance/internal/rule/matrix.go.

package app

import (
	"errors"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Mechanism is where a contained message goes. Quarantine holds it for
// inspection with no automatic redelivery; the dead-letter queue is the
// transport's terminal destination and belongs to KRN-10 (GAR-11).
type Mechanism string

const (
	MechanismQuarantine Mechanism = "quarantine"
	MechanismDeadLetter Mechanism = "dead-letter"
)

// ErrUnsupportedMechanism is what Consume reports if the declared mechanism has
// no realization in this module — today only quarantine has one.
var ErrUnsupportedMechanism = errors.New("app: containment mechanism has no realization here")

// ContainmentMap is the declaration GAR-11 requires: which situation goes to
// which mechanism. Attempts-exhausted moves to the transport's dead-letter
// queue once KRN-10 exists; until then every reason is quarantined.
var ContainmentMap = map[ports.Reason]Mechanism{
	ports.ReasonInvalidEnvelope:   MechanismQuarantine,
	ports.ReasonTerminalFailure:   MechanismQuarantine,
	ports.ReasonCollision:         MechanismQuarantine,
	ports.ReasonAttemptsExhausted: MechanismQuarantine,
	ports.ReasonUntrustedBoundary: MechanismQuarantine,
}

// MechanismFor resolves the declared mechanism; ok is false for a reason the
// port does not enumerate, which the adapter treats as a defect rather than a
// default route.
func MechanismFor(reason ports.Reason) (Mechanism, bool) {
	mechanism, ok := ContainmentMap[reason]
	return mechanism, ok
}
