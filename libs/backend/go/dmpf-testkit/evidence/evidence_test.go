package evidence_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/serviceskit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/tb"
)

type spy struct {
	testing.TB
	fatal string
}

func (s *spy) Helper() {}
func (s *spy) Fatalf(format string, args ...any) {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, format, args...)
	s.fatal = b.String()
}

func readRecord(t *testing.T, dir, subject, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, subject, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestRecordWithoutTheVariableWritesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv(evidence.DirEnv, "")

	evidence.RecordVerdict(t, "services", "accepted", serviceskit.Verdict{})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("wrote %d entr(y/ies) with %s empty", len(entries), evidence.DirEnv)
	}
}

func TestRecordReportWritesTheReportsStableOrder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(evidence.DirEnv, dir)
	var r golden.Report
	r.Add(
		golden.Outcome{Fixture: "reservations/placed", Case: "canonical", OK: true},
		golden.Outcome{Fixture: "orders/item-added", Case: "canonical", OK: true},
	)

	evidence.RecordReport(t, "golden", "orders", r)

	want, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if got := readRecord(t, dir, "golden", "orders"); got != string(want) {
		t.Fatalf("record = %s\nwant    %s", got, want)
	}
}

func TestRecordVerdictSortsWhatEveryKitReports(t *testing.T) {
	cases := []struct {
		name    string
		verdict tb.Verdict
		want    string
	}{
		{
			name:    "pass",
			verdict: domainkit.Verdict{},
			want:    `{"ok":true,"failures":[],"skipped":[]}`,
		},
		{
			name: "domain",
			verdict: domainkit.Verdict{Diagnostics: []domainkit.Diagnostic{
				{Code: domainkit.CodeSecondRead, Field: "state", Expected: "{}", Got: "{x=1}"},
				{Code: domainkit.CodeProjection, Field: "branch", Expected: "accepted", Got: "rejected"},
			}},
			want: `{"ok":false,"failures":["ORA-31 branch: expected accepted, got rejected","ORA-38 state: expected {}, got {x=1}"],"skipped":[]}`,
		},
		{
			name: "services",
			verdict: serviceskit.Verdict{Diagnostics: []serviceskit.Diagnostic{
				{Rule: "UOW-08", Seq: 1, Detail: "published"},
				{Rule: "UOW-06", Seq: -1, Detail: "2 committed transactions"},
			}},
			want: `{"ok":false,"failures":["UOW-06: 2 committed transactions","UOW-08 at ledger position 1: published"],"skipped":[]}`,
		},
		{
			name: "provider",
			verdict: providerkit.Verdict{
				Diagnostics: []providerkit.Diagnostic{
					{Clause: "outbox-order", Rule: "OBX-11", Detail: "entries out of order"},
					{Clause: "inbox-dedup", Rule: "INB-06", Detail: "duplicate accepted"},
				},
				Skipped: []string{"uow-commit-failure", "inbox-lock-timeout"},
			},
			want: `{"ok":false,"failures":["inbox-dedup [INB-06]: duplicate accepted","outbox-order [OBX-11]: entries out of order"],"skipped":["inbox-lock-timeout","uow-commit-failure"]}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv(evidence.DirEnv, dir)

			evidence.RecordVerdict(t, "kits", c.name, c.verdict)

			if got := readRecord(t, dir, "kits", c.name); got != c.want {
				t.Fatalf("record = %s\nwant    %s", got, c.want)
			}
		})
	}
}

func TestRecordingTheSameNameTwiceFails(t *testing.T) {
	t.Setenv(evidence.DirEnv, t.TempDir())
	s := &spy{TB: t}

	evidence.RecordVerdict(s, "services", "accepted", serviceskit.Verdict{})
	if s.fatal != "" {
		t.Fatalf("first record failed: %s", s.fatal)
	}
	evidence.RecordVerdict(s, "services", "accepted", serviceskit.Verdict{})

	if s.fatal == "" {
		t.Fatal("a second record under the same name replaced the first")
	}
}

func TestASubjectOrNameThatLeavesTheDirectoryIsRefused(t *testing.T) {
	root := t.TempDir()
	t.Setenv(evidence.DirEnv, filepath.Join(root, "evidence"))
	cases := []struct{ subject, name string }{
		{"services", "../escaped"},
		{"services", "a/b"},
		{"services", ""},
		{"..", "escaped"},
		{"", "escaped"},
	}
	for _, c := range cases {
		s := &spy{TB: t}
		evidence.RecordVerdict(s, c.subject, c.name, serviceskit.Verdict{})
		if s.fatal == "" {
			t.Errorf("subject %q name %q was accepted", c.subject, c.name)
		}
	}
	for _, leaked := range []string{"escaped.json", "evidence/escaped.json"} {
		if _, err := os.Stat(filepath.Join(root, leaked)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s exists after a refused record (err = %v)", leaked, err)
		}
	}
}
