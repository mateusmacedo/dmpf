package tb

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/golden"
)

// RepoRoot is the directory that holds go.work, found by climbing from this
// file: a kit test runs from its own package directory, and the workspace root
// is what the fitness function and the fixtures under contracts/ are relative to.
func RepoRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("tb.RepoRoot: runtime.Caller gave no file")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("tb.RepoRoot: no go.work above %s", file)
		}
		dir = parent
	}
}

// ReadFixture reads a file by its path relative to the repository root, which
// is where contracts/ keeps every golden fixture (FIX-10). A path that leaves
// the repository is refused: fixtures are versioned content, nothing else.
func ReadFixture(t testing.TB, rel string) []byte {
	t.Helper()
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		t.Fatalf("tb.ReadFixture: %q escapes the repository", rel)
	}
	raw, err := os.ReadFile(filepath.Join(RepoRoot(t), clean))
	if err != nil {
		t.Fatalf("tb.ReadFixture: %v", err)
	}
	return raw
}

// Env resolves what an integration harness needs from the environment. Unset,
// it skips, naming the variable — except in CI, where it fails: an integration
// suite must never pass by skipping (fail-closed, as the Postgres suite does).
func Env(t testing.TB, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v != "" {
		return v
	}
	if os.Getenv("CI") != "" {
		t.Fatalf("%s is empty in CI: integration tests must not skip silently", name)
	}
	t.Skipf("%s not set", name)
	return ""
}

// Verdict is what every kit decides: a list of failures, empty on a pass. The
// kits keep their own verdict types; this is the shape tb needs to fail a test.
type Verdict interface{ Failures() []string }

// Skipper is what a verdict with clauses it could not exercise also offers;
// Require logs them so a skip is never silent.
type Skipper interface{ Skips() []string }

func Require(t testing.TB, v Verdict) {
	t.Helper()
	for _, f := range v.Failures() {
		t.Errorf("%s", f)
	}
	if s, ok := v.(Skipper); ok {
		for _, clause := range s.Skips() {
			t.Logf("clause not exercised by this candidate: %s", clause)
		}
	}
}

// RequireReport fails the test once per failed outcome, so every oracle that
// reproved is on the log, never only the first (ORA-06).
func RequireReport(t testing.TB, r golden.Report) {
	t.Helper()
	for _, o := range r.Failed() {
		t.Errorf("%s %s/%s %s oracle %d: field %s expected %q got %q", o.Code, o.Fixture, o.Case, o.Direction, o.Oracle, o.Field, o.Expected, o.Got)
	}
}
