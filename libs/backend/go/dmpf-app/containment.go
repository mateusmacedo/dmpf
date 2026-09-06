// comment-discipline-ok-file: arquivo de declarações; o mapeamento situação → mecanismo é norma revisável em PR (GAR-11), no molde de dmpf-conformance/internal/rule/matrix.go.

package dmpfapp

import (
	"errors"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
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
var ErrUnsupportedMechanism = errors.New("dmpfapp: containment mechanism has no realization here")

// ContainmentMap is the declaration GAR-11 requires: which situation goes to
// which mechanism. Attempts-exhausted moves to the transport's dead-letter
// queue once KRN-10 exists; until then every reason is quarantined.
var ContainmentMap = map[dmpfports.Reason]Mechanism{
	dmpfports.ReasonInvalidEnvelope:   MechanismQuarantine,
	dmpfports.ReasonTerminalFailure:   MechanismQuarantine,
	dmpfports.ReasonCollision:         MechanismQuarantine,
	dmpfports.ReasonAttemptsExhausted: MechanismQuarantine,
}

// MechanismFor resolves the declared mechanism; ok is false for a reason the
// port does not enumerate, which the adapter treats as a defect rather than a
// default route.
func MechanismFor(reason dmpfports.Reason) (Mechanism, bool) {
	mechanism, ok := ContainmentMap[reason]
	return mechanism, ok
}
