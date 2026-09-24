package evidence

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const DirEnv = "DMPF_EVIDENCE_DIR"

func RecordReport(t testing.TB, subject, name string, r golden.Report) {
	t.Helper()
	record(t, subject, name, r)
}

type verdictRecord struct {
	OK       bool     `json:"ok"`
	Failures []string `json:"failures"`
	Skipped  []string `json:"skipped"`
}

// RecordVerdict sorts failures and skipped clauses: the record must not depend
// on the order in which a kit happened to append its diagnostics.
func RecordVerdict(t testing.TB, subject, name string, v tb.Verdict) {
	t.Helper()
	rec := verdictRecord{Failures: sortedCopy(v.Failures()), Skipped: []string{}}
	rec.OK = len(rec.Failures) == 0
	if s, ok := v.(tb.Skipper); ok {
		rec.Skipped = sortedCopy(s.Skips())
	}
	record(t, subject, name, rec)
}

func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	slices.Sort(out)
	return out
}

func record(t testing.TB, subject, name string, value any) {
	t.Helper()
	if !isSegment(subject) || !isSegment(name) {
		t.Fatalf("evidence: subject %q and name %q must each be a single path segment", subject, name)
		return
	}
	root := os.Getenv(DirEnv)
	if root == "" {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("evidence: %s/%s: %v", subject, name, err)
		return
	}
	dir := filepath.Join(root, subject)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("evidence: %s/%s: %v", subject, name, err)
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, name+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		t.Fatalf("evidence: %s/%s recorded twice in the same run", subject, name)
		return
	}
	if err != nil {
		t.Fatalf("evidence: %s/%s: %v", subject, name, err)
		return
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		t.Fatalf("evidence: %s/%s: %v", subject, name, err)
		return
	}
	if err := f.Close(); err != nil {
		t.Fatalf("evidence: %s/%s: %v", subject, name, err)
	}
}

func isSegment(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, `/\`)
}
