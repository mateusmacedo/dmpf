package fitness_test

import (
	"testing"

	conffit "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/fitness"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
)

// FIT-01: the assertion is over the production universe of the workspace as
// the compiler resolves it, reusing the checker's diagnostics (FIT-02) and
// nothing of its trust model (FIT-03).
func TestNenhumaArestaProibida(t *testing.T) {
	in, err := conffit.Workspace(tb.RepoRoot(t), "")
	if err != nil {
		t.Fatalf("Workspace: %v", err)
	}
	ds, err := conffit.Diagnostics(in)
	if err != nil {
		t.Fatalf("Diagnostics: %v", err)
	}
	for _, d := range ds {
		t.Errorf("%s %s -> %s (%s): %s", d.Code, d.CanonicalKey, d.Target, d.SourceFile, d.Detail)
	}
}
