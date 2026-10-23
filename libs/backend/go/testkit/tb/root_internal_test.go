package tb

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func workspace(t *testing.T, dirs ...string) string {
	t.Helper()
	dir := filepath.Join(dirs...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.work"), []byte("go 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func mkdir(t *testing.T, dirs ...string) string {
	t.Helper()
	dir := filepath.Join(dirs...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func requireNoWorkspaceAboveTemp(t *testing.T) {
	t.Helper()
	if root, err := findRoot([]string{t.TempDir()}); err == nil {
		t.Skipf("the go.work at %s above the temporary directory makes a start without one impossible", root)
	}
}

func TestFindRootClimbsEachStartInOrder(t *testing.T) {
	t.Run("a start with no go.work above falls through to the next", func(t *testing.T) {
		requireNoWorkspaceAboveTemp(t)
		root := workspace(t, t.TempDir())
		lost := mkdir(t, t.TempDir(), "a", "b")
		got, err := findRoot([]string{lost, mkdir(t, root, "libs", "kit")})
		if err != nil || got != root {
			t.Fatalf("findRoot = %q, %v; want %q", got, err, root)
		}
	})

	t.Run("the nearest go.work wins over an ancestor's", func(t *testing.T) {
		outer := workspace(t, t.TempDir())
		inner := workspace(t, outer, "nested")
		got, err := findRoot([]string{inner})
		if err != nil || got != inner {
			t.Fatalf("findRoot = %q, %v; want %q", got, err, inner)
		}
	})

	t.Run("no start with a go.work above names every start", func(t *testing.T) {
		requireNoWorkspaceAboveTemp(t)
		first, second := mkdir(t, t.TempDir(), "x"), mkdir(t, t.TempDir(), "y")
		_, err := findRoot([]string{first, second})
		if err == nil {
			t.Fatal("findRoot found a root where none exists")
		}
		want := "no go.work above " + first + " nor " + second
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	})
}

func TestRepoRootFallsBackToTheKitFile(t *testing.T) {
	requireNoWorkspaceAboveTemp(t)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller gave no file for this test")
	}
	want, err := findRoot([]string{filepath.Dir(file)})
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if got := RepoRoot(t); got != want {
		t.Fatalf("RepoRoot = %q, want %q", got, want)
	}
}
