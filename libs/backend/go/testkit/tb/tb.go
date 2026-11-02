package tb

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
)

// RepoRoot is the directory that holds go.work, the root the fitness function
// and every fixture path are relative to. It climbs first from the test's
// working directory: go test runs in the package directory, which lies inside
// the workspace of whoever runs the test, including a consumer that takes the
// kit by version from the module cache. Only when no go.work is found there
// does it climb from this file, which covers a test binary run from elsewhere.
func RepoRoot(t testing.TB) string {
	t.Helper()
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		starts = append(starts, filepath.Dir(file))
	}
	if len(starts) == 0 {
		t.Fatal("tb.RepoRoot: neither the working directory nor runtime.Caller gave a place to start")
	}
	root, err := findRoot(starts)
	if err != nil {
		t.Fatalf("tb.RepoRoot: %v", err)
	}
	return root
}

func findRoot(starts []string) (string, error) {
	for _, start := range starts {
		for dir := start; ; {
			if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", fmt.Errorf("no go.work above %s", strings.Join(starts, " nor "))
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
