//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/tb"
)

func runCommand(t *testing.T, args ...string) (int, string) {
	t.Helper()
	root := tb.RepoRoot(t)
	pinWorkspaceToolchain(t, root)
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), append([]string{"--root", root, "--allow-dirty"}, args...), &stdout, &stderr)
	return code, stderr.String()
}

// pinWorkspaceToolchain mirrors the evidence target and CI, which run with the
// go.work toolchain: the command refuses any other, whatever runs the test.
func pinWorkspaceToolchain(t *testing.T, root string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if version, ok := strings.CutPrefix(strings.TrimSpace(line), "go "); ok {
			t.Setenv("GOTOOLCHAIN", "go"+strings.TrimSpace(version))
			return
		}
	}
	t.Fatal("go.work declares no go line")
}

func readTree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	tree := map[string][]byte{}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		tree[e.Name()] = raw
	}
	return tree
}

func TestTwoRunsOverTheSameCommitPublishTheSameBytes(t *testing.T) {
	t.Setenv("CI", "")
	outs := []string{filepath.Join(t.TempDir(), "0.1.0"), filepath.Join(t.TempDir(), "0.1.0")}
	for _, out := range outs {
		if code, stderr := runCommand(t, "--release", "latest", "--subjects", "golden,domain,services", "--out", out); code != exitOK {
			t.Fatalf("exit %d: %s", code, stderr)
		}
	}

	first, second := readTree(t, outs[0]), readTree(t, outs[1])
	names := []string{"domain.json", "golden.json", "index.json", "services.json"}
	for _, tree := range []map[string][]byte{first, second} {
		if got := slices.Sorted(maps.Keys(tree)); !slices.Equal(got, names) {
			t.Fatalf("published files = %v, want %v", got, names)
		}
	}
	for _, name := range names {
		if !bytes.Equal(first[name], second[name]) {
			t.Fatalf("%s differs between two runs over the same commit", name)
		}
	}
}

func TestGoldenInCIIsNotReprovedByTheFixtureMaintenanceTest(t *testing.T) {
	t.Setenv("CI", "true")
	out := filepath.Join(t.TempDir(), "0.1.0")

	if code, stderr := runCommand(t, "--release", "latest", "--subjects", "golden", "--out", out); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	var subject evidence.Subject
	if err := json.Unmarshal(readTree(t, out)["golden.json"], &subject); err != nil {
		t.Fatal(err)
	}
	if subject.Body == nil || slices.ContainsFunc(subject.Body.Tests, func(r evidence.TestResult) bool { return r.Test == "TestUpdateGolden" }) {
		t.Fatalf("golden body = %+v, want the tests without TestUpdateGolden", subject.Body)
	}
}

func TestProviderEvidenceNamesThePostgresItRanAgainst(t *testing.T) {
	tb.Env(t, "DMPF_PG_DSN")
	out := filepath.Join(t.TempDir(), "0.1.0")
	if code, stderr := runCommand(t, "--release", "latest", "--subjects", "provider", "--out", out); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var subject evidence.Subject
	if err := json.Unmarshal(readTree(t, out)["provider.json"], &subject); err != nil {
		t.Fatal(err)
	}
	if subject.Header.Infra["postgres"] == "" || subject.Header.Infra["redpanda"] != "" {
		t.Fatalf("infra = %v, want postgres only", subject.Header.Infra)
	}
	if !slices.ContainsFunc(subject.Header.Externals, func(e evidence.External) bool { return e.Package == "github.com/jackc/pgx/v5" }) {
		t.Fatalf("the provider header does not list pgx: %+v", subject.Header.Externals)
	}
}

func TestDistWithoutBrokersInCIExitsNamingTheVariable(t *testing.T) {
	t.Setenv("CI", "true")
	t.Setenv("DMPF_PG_DSN", "postgres://unused")
	t.Setenv("DMPF_KAFKA_BROKERS", "")
	out := filepath.Join(t.TempDir(), "0.1.0")

	code, stderr := runCommand(t, "--release", "latest", "--subjects", "dist", "--out", out)

	if code != exitReproved || !strings.Contains(stderr, "DMPF_KAFKA_BROKERS") {
		t.Fatalf("exit %d, stderr %q; want exit %d naming DMPF_KAFKA_BROKERS", code, stderr, exitReproved)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("%s was published (err = %v)", out, err)
	}
}

func TestDistWithoutBrokersOutsideCIIsRecordedAsSkippedAndNotIndexed(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("DMPF_KAFKA_BROKERS", "")
	out := filepath.Join(t.TempDir(), "0.1.0")

	if code, stderr := runCommand(t, "--release", "latest", "--subjects", "dist", "--out", out); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	tree := readTree(t, out)
	var subject evidence.Subject
	if err := json.Unmarshal(tree["dist.json"], &subject); err != nil {
		t.Fatal(err)
	}
	if subject.Skipped == "" || subject.Body != nil || len(subject.Header.Externals) != 0 {
		t.Fatalf("dist = %+v, want skipped with no body and no reach", subject)
	}
	if strings.TrimSpace(string(tree["index.json"])) != "[]" {
		t.Fatalf("index.json = %s, want []", tree["index.json"])
	}
}

func TestInvocationErrorsExitWithUsage(t *testing.T) {
	existing := t.TempDir()
	fresh := filepath.Join(t.TempDir(), "0.1.0")
	cases := map[string][]string{
		"missing release":  {"--out", fresh},
		"missing out":      {"--release", "latest"},
		"unknown release":  {"--release", "9.9.9", "--out", fresh},
		"unknown subject":  {"--release", "latest", "--subjects", "nope", "--out", fresh},
		"existing out":     {"--release", "latest", "--out", existing},
		"stray positional": {"--release", "latest", "--out", fresh, "extra"},
	}
	for name, args := range cases {
		if code, stderr := runCommand(t, args...); code != exitUsage {
			t.Errorf("%s: exit %d (%s), want %d", name, code, stderr, exitUsage)
		}
	}
}
