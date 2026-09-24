package fitness_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/fitness"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

var (
	contextFields = []string{"request_id", "correlation_id", "causation_id", "trace_context",
		"authenticated_subject", "tenant_id", "permissions", "deadline", "locale"}
	boundaries = []fitness.Boundary{fitness.Ingress, fitness.FanOut, fitness.Retry, fitness.Downstream}
	actions    = map[fitness.Action]bool{fitness.Preserve: true, fitness.Regenerate: true, fitness.Reject: true, fitness.Reduce: true}

	// envelopeNames are the fields a reduced value would take in the envelope;
	// the CloudEvents Subject is the aggregate, not the authenticated subject.
	envelopeNames = map[string][]string{
		"request_id":            {"RequestID"},
		"authenticated_subject": {"AuthenticatedSubject", "SubjectID"},
		"permissions":           {"Permissions"},
		"deadline":              {"Deadline"},
		"locale":                {"Locale"},
	}
)

// CTX-11: every field resolves exactly one of the four actions at each of the
// four boundaries; a crossing without an action is a defect, not an omission.
func TestEveryFieldResolvesExactlyOneActionAtEveryBoundary(t *testing.T) {
	cells := map[string]int{}
	for _, c := range fitness.Traversal {
		if !actions[c.Action] {
			t.Errorf("%s at %s: %q is not one of the four actions", c.Field, c.Boundary, c.Action)
		}
		cells[c.Field+"@"+string(c.Boundary)]++
	}
	for _, field := range contextFields {
		for _, boundary := range boundaries {
			if n := cells[field+"@"+string(boundary)]; n != 1 {
				t.Errorf("%s at %s resolves %d actions, want exactly 1", field, boundary, n)
			}
		}
	}
	if len(fitness.Traversal) != len(contextFields)*len(boundaries) {
		t.Errorf("%d crossings, want %d", len(fitness.Traversal), len(contextFields)*len(boundaries))
	}
}

// Every cell names where its realization is executed, and the name has to
// still exist: a renamed or deleted test leaves the cell unproven, which fails
// here instead of rotting in the table.
func TestEveryCrossingIsProvenByATestThatExists(t *testing.T) {
	root := tb.RepoRoot(t)
	for _, c := range fitness.Traversal {
		if c.ProofEnvelope {
			continue
		}
		path, test, ok := strings.Cut(c.Proof, "::")
		if !ok || test == "" {
			t.Errorf("%s at %s: proof %q is not path::Test", c.Field, c.Boundary, c.Proof)
			continue
		}
		source, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Errorf("%s at %s: %v", c.Field, c.Boundary, err)
			continue
		}
		if !strings.Contains(string(source), "func "+test+"(") {
			t.Errorf("%s at %s: %s declares no %s", c.Field, c.Boundary, path, test)
		}
	}
}

// The reductions at the fan-out are structural: the envelope has no attribute
// that could carry the value, so no producer can put it on the wire (CTX-12).
func TestTheEnvelopeCarriesNoFieldTheFanOutReduces(t *testing.T) {
	envelopeType := reflect.TypeOf(envelope.Envelope{})
	for _, c := range fitness.Traversal {
		if !c.ProofEnvelope {
			continue
		}
		if c.Boundary != fitness.FanOut || c.Action != fitness.Reduce {
			t.Errorf("%s at %s: a structural proof only stands for a reduction at the fan-out", c.Field, c.Boundary)
			continue
		}
		names, known := envelopeNames[c.Field]
		if !known {
			t.Errorf("%s: no envelope name registered to check", c.Field)
			continue
		}
		for _, name := range names {
			if _, found := envelopeType.FieldByName(name); found {
				t.Errorf("%s: envelope.Envelope has %s, so the fan-out does not reduce it", c.Field, name)
			}
		}
	}
}
