package fitness_test

import (
	"sync"
	"testing"
	"time"

	conffit "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-conformance/fitness"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
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
	shared.once.Do(func() { shared.in, shared.err = conffit.Workspace(tb.RepoRoot(t), "") })
	if shared.err != nil {
		t.Fatalf("Workspace: %v", shared.err)
	}
	return shared.in
}
