package fitness_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	conffit "github.com/mateusmacedo/dmpf/tools/dmpf-conformance/fitness"
)

// goListTimeout bounds every go list the suite spawns: a module download that
// stalls fails the test by name instead of running into go test's -timeout.
const goListTimeout = 2 * time.Minute

var shared struct {
	once sync.Once
	in   conffit.Input
	err  error
}

// workspace resolves the production universe once per test binary: building
// it is one go list per module and per build profile, and every test in the
// package reads the same universe.
func workspace(t *testing.T) conffit.Input {
	t.Helper()
	shared.once.Do(func() {
		shared.in, shared.err = conffit.Workspace(tb.RepoRoot(t), "")
		if shared.err == nil {
			shared.in.SharedKernelUnits, shared.err = sharedKernelUnits(tb.RepoRoot(t))
		}
	})
	if shared.err != nil {
		t.Fatalf("Workspace: %v", shared.err)
	}
	return shared.in
}

// sharedKernelUnits reads only the designation from the governed baseline, and
// never its classification: the suite keeps asserting over the universe the
// compiler resolves, with no BaselineStore and no trust model (FIT-03). Reading
// the list instead of restating it here is what keeps the two from drifting —
// a designation changed in the baseline must not leave a stale copy in a test.
func sharedKernelUnits(root string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(root, "tools", "dmpf-baseline", "units-baseline.json"))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Units []string `json:"shared_kernel_units"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return doc.Units, nil
}
