package fitness_test

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
)

var (
	exactlyOnce = regexp.MustCompile(`(?i)exactly[- ]once`)
	// A line that names exactly-once only to forbid it is the positive of V31.
	forbidsIt = regexp.MustCompile(`(?i)veda|não promete|at-least-once|at least once|nunca promet`)
)

// V31 (P0-3, RAS-12): no document, contract or configuration promises
// exactly-once end to end. Structurally reviewable, so the instrument is a
// sweep, not a test of behaviour.
func TestV31NoArtifactPromisesExactlyOnce(t *testing.T) {
	root := tb.RepoRoot(t)
	var files []string
	files = append(files, sweep(t, filepath.Join(root, "contracts"), func(string) bool { return true })...)
	files = append(files, sweep(t, filepath.Join(root, "libs", "backend", "go"), func(p string) bool {
		base := filepath.Base(p)
		return base == "README.md" || base == "dmpf-units.json"
	})...)
	for _, f := range files {
		for _, hit := range promises(t, f) {
			t.Errorf("%s: promete exactly-once fim a fim: %s", hit.where, hit.line)
		}
	}
}

func TestV31NamesAPromiseAndAcceptsAProhibition(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte("A entrega é exactly-once fim a fim.\nExactly-once é vedada como promessa (P0-3).\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hits := promises(t, path)
	if len(hits) != 1 || !strings.HasSuffix(hits[0].where, "README.md:1") {
		t.Fatalf("hits = %+v, want exactly line 1", hits)
	}
}

type hit struct{ where, line string }

func promises(t *testing.T, path string) []hit {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []hit
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if exactlyOnce.MatchString(line) && !forbidsIt.MatchString(line) {
			out = append(out, hit{where: path + ":" + strconv.Itoa(n), line: strings.TrimSpace(line)})
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return out
}

func sweep(t *testing.T, root string, keep func(string) bool) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "testdata", ".git", "gen":
				return filepath.SkipDir
			}
			return nil
		}
		if keep(path) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return out
}
