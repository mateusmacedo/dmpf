package evidence_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
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

func goTestStream(stamp, elapsed string) string {
	stream := strings.Join([]string{
		`{"Time":"{T}","Action":"start","Package":"example.com/b"}`,
		`{"Time":"{T}","Action":"run","Package":"example.com/b","Test":"TestZ"}`,
		`{"Time":"{T}","Action":"output","Package":"example.com/b","Test":"TestZ","Output":"=== RUN   TestZ\n"}`,
		`{"Time":"{T}","Action":"pass","Package":"example.com/b","Test":"TestZ","Elapsed":{E}}`,
		`{"Time":"{T}","Action":"skip","Package":"example.com/a","Test":"TestY","Elapsed":{E}}`,
		`{"Time":"{T}","Action":"pass","Package":"example.com/b","Elapsed":{E}}`,
		`{"Time":"{T}","Action":"fail","Package":"example.com/a","Test":"TestX","Elapsed":{E}}`,
	}, "\n")
	return strings.ReplaceAll(strings.ReplaceAll(stream, "{T}", stamp), "{E}", elapsed)
}

func TestFilterStreamKeepsOnlyTerminalActionsInOrder(t *testing.T) {
	got, err := evidence.FilterStream(strings.NewReader(goTestStream("2026-09-13T13:00:00.1-03:00", "0.01")))
	if err != nil {
		t.Fatal(err)
	}
	want := []evidence.TestResult{
		{Package: "example.com/a", Test: "TestX", Action: "fail"},
		{Package: "example.com/a", Test: "TestY", Action: "skip"},
		{Package: "example.com/b", Action: "pass"},
		{Package: "example.com/b", Test: "TestZ", Action: "pass"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("filtered = %+v\nwant       %+v", got, want)
	}
}

func TestFilterStreamIsIndependentOfTimeAndElapsed(t *testing.T) {
	first, err := evidence.FilterStream(strings.NewReader(goTestStream("2026-09-13T13:00:00.1-03:00", "0.01")))
	if err != nil {
		t.Fatal(err)
	}
	second, err := evidence.FilterStream(strings.NewReader(goTestStream("2027-01-01T00:00:59.9Z", "7.5")))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("two runs differ only in time and still serialize apart:\n%s\n%s", a, b)
	}
	for _, key := range []string{"time", "elapsed", "output"} {
		if strings.Contains(strings.ToLower(string(a)), `"`+key+`"`) {
			t.Fatalf("%q survived the filter: %s", key, a)
		}
	}
}

func TestFilterStreamRefusesALineThatIsNotAnEvent(t *testing.T) {
	if _, err := evidence.FilterStream(strings.NewReader("# example.com/a\nnot json\n")); err == nil {
		t.Fatal("a non-JSON line was accepted")
	}
}

func TestJudgeResultsReprovesFailuresAlwaysAndSkipsOnlyInCI(t *testing.T) {
	passing := []evidence.TestResult{
		{Package: "p", Test: "TestA", Action: "pass"},
		{Package: "p", Test: "TestB", Action: "skip"},
	}
	if got := evidence.JudgeResults(passing, false); len(got) != 0 {
		t.Fatalf("outside CI a skip reproved: %v", got)
	}
	if got := evidence.JudgeResults(passing, true); len(got) != 1 || !strings.Contains(got[0], "TestB") {
		t.Fatalf("in CI the skip of TestB was not named: %v", got)
	}
	failing := append(slices.Clone(passing), evidence.TestResult{Package: "q", Action: "fail"})
	if got := evidence.JudgeResults(failing, false); len(got) != 1 || !strings.Contains(got[0], "q") {
		t.Fatalf("the failed package was not named: %v", got)
	}
}

func TestRunGoTestLeavesOnlyTheExactlyNamedTestsOutOfTheStream(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/skip\n\ngo 1.26\n",
		"skip_test.go": "package skip\n\nimport \"testing\"\n\n" +
			"func TestKept(t *testing.T) {}\n\n" +
			"func TestMaintenance(t *testing.T) { t.Skip(\"maintenance only\") }\n\n" +
			"func TestMaintenanceAll(t *testing.T) { t.Skip(\"maintenance only\") }\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := evidence.RunGoTest(context.Background(), evidence.GoTest{
		Dir:      dir,
		Packages: []string{"."},
		Skip:     []string{"TestMaintenance"},
		Env:      []string{"GOWORK=off"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if slices.ContainsFunc(results, func(r evidence.TestResult) bool { return r.Test == "TestMaintenance" }) {
		t.Fatalf("the excluded test reached the stream: %+v", results)
	}
	if !slices.Contains(results, evidence.TestResult{Package: "example.com/skip", Test: "TestKept", Action: "pass"}) {
		t.Fatalf("TestKept did not pass: %+v", results)
	}
	if got := evidence.JudgeResults(results, true); len(got) != 1 || !strings.Contains(got[0], "TestMaintenanceAll") {
		t.Fatalf("in CI only TestMaintenanceAll should reprove, got %v", got)
	}
}

func TestGoVersionRefusesAToolchainOtherThanTheWorkspaces(t *testing.T) {
	work := []byte("go 1.26.6\n\nuse (\n\t./a\n)\n")
	if got, err := evidence.GoVersion(work, "go1.26.6"); err != nil || got != "1.26.6" {
		t.Fatalf("GoVersion = %q, %v; want 1.26.6", got, err)
	}
	if _, err := evidence.GoVersion(work, "go1.27.1"); err == nil {
		t.Fatal("a go1.27.1 toolchain passed for a go 1.26.6 workspace")
	}
	if _, err := evidence.GoVersion([]byte("use ./a\n"), "go1.26.6"); err == nil {
		t.Fatal("a go.work without a go line passed")
	}
}

func TestReachListsOnlyWhatTheSubjectImports(t *testing.T) {
	root := tb.RepoRoot(t)
	modules, externals, err := evidence.Reach(context.Background(), root, nil,
		[]string{"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/golden"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "libs/backend/go/dmpf-contracts/package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct{ Version string }
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	contracts := evidence.Module{Path: "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts", Version: pkg.Version}
	if !slices.Contains(modules, contracts) {
		t.Fatalf("modules = %+v, want %+v", modules, contracts)
	}
	if !slices.ContainsFunc(externals, func(e evidence.External) bool {
		return e.Package == "google.golang.org/protobuf" && e.Version != ""
	}) {
		t.Fatalf("externals = %+v, want google.golang.org/protobuf with its resolved version", externals)
	}
	for _, e := range externals {
		if strings.HasPrefix(e.Package, "github.com/twmb/franz-go") || e.Package == "github.com/jackc/pgx/v5" {
			t.Fatalf("golden does not import %s and the header lists it: %+v", e.Package, externals)
		}
	}
	if !slices.IsSortedFunc(modules, func(a, b evidence.Module) int { return strings.Compare(a.Path, b.Path) }) ||
		!slices.IsSortedFunc(externals, func(a, b evidence.External) int { return strings.Compare(a.Package, b.Package) }) {
		t.Fatalf("reach is not sorted: %+v %+v", modules, externals)
	}
}

func TestParseRedpandaBrokersTakesTheReleaseOfTheFirstBroker(t *testing.T) {
	raw := []byte(`[{"node_id":0,"version":"v26.2.2 - 551a6866f4804ee5753e3ffb5953a3355733e78b"}]`)
	if got, err := evidence.ParseRedpandaBrokers(raw); err != nil || got != "v26.2.2" {
		t.Fatalf("ParseRedpandaBrokers = %q, %v; want v26.2.2", got, err)
	}
	for _, bad := range []string{`[]`, `[{"node_id":0}]`, `{"version":"v26.2.2"}`} {
		if _, err := evidence.ParseRedpandaBrokers([]byte(bad)); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
}

func TestToolVersionsReadThePinnedRegistries(t *testing.T) {
	bufScript := []byte("#!/usr/bin/env bash\nexec go run github.com/bufbuild/buf/cmd/buf@v1.72.0 \"$@\"\n")
	bufGen := []byte("plugins:\n  - local:\n      - go\n      - run\n      - google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12\n")
	got, err := evidence.ToolVersions(bufScript, bufGen)
	if err != nil {
		t.Fatal(err)
	}
	want := []evidence.Tool{
		{Identity: "github.com/bufbuild/buf/cmd/buf", Version: "v1.72.0"},
		{Identity: "google.golang.org/protobuf/cmd/protoc-gen-go", Version: "v1.36.12"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tools = %+v, want %+v", got, want)
	}
	if _, err := evidence.ToolVersions([]byte("exec buf \"$@\"\n"), bufGen); err == nil {
		t.Fatal("a buf.sh without a pin passed")
	}
	if _, err := evidence.ToolVersions(bufScript, []byte("plugins: []\n")); err == nil {
		t.Fatal("a buf.gen.yaml without the protoc-gen-go pin passed")
	}
}

func sampleSubject(t *testing.T, name string, outcomes int) evidence.Subject {
	t.Helper()
	var r golden.Report
	for i := range outcomes {
		r.Add(golden.Outcome{Fixture: "orders/event/v1/item-added", Case: fmt.Sprintf("case-%d", i), OK: true})
	}
	raw, err := r.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return evidence.Subject{
		Header: evidence.Header{Schema: evidence.Schema, Release: "0.1.0", Subject: name, GoVersion: "1.26.6"},
		Body: &evidence.Body{
			Tests:   []evidence.TestResult{{Package: "example.com/golden", Test: "TestGoldenRoundTrip", Action: "pass"}},
			Records: map[string]json.RawMessage{"orders-event-v1-item-added": raw},
		},
	}
}

func TestEncodeIsStableAndTheDigestFollowsTheContent(t *testing.T) {
	first, err := evidence.Encode(sampleSubject(t, "golden", 3))
	if err != nil {
		t.Fatal(err)
	}
	second, err := evidence.Encode(sampleSubject(t, "golden", 3))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Digest(first) != evidence.Digest(second) {
		t.Fatal("two encodings of the same subject digest apart")
	}
	fewer, err := evidence.Encode(sampleSubject(t, "golden", 2))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Digest(fewer) == evidence.Digest(first) {
		t.Fatal("an outcome less did not change the digest")
	}
	if !strings.HasPrefix(evidence.Digest(first), "sha256:") || len(evidence.Digest(first)) != len("sha256:")+64 {
		t.Fatalf("digest = %q, want sha256:<64 hex>", evidence.Digest(first))
	}
}

func TestPublishRefusesASetOtherThanTheCatalog(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "0.1.0")
	cases := map[string][]evidence.Subject{
		"missing":   {sampleSubject(t, "golden", 1)},
		"unknown":   {sampleSubject(t, "golden", 1), sampleSubject(t, "domain", 1), sampleSubject(t, "extra", 1)},
		"duplicate": {sampleSubject(t, "golden", 1), sampleSubject(t, "golden", 1)},
	}
	for name, subjects := range cases {
		if _, err := evidence.Publish(dest, []string{"domain", "golden"}, subjects); err == nil {
			t.Errorf("%s: a set other than the catalog was published", name)
		}
		if _, err := os.Stat(dest); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s: the destination exists after a refused publication (err = %v)", name, err)
		}
	}
}

func TestPublishRefusesAnExistingDestination(t *testing.T) {
	dest := t.TempDir()
	kept := filepath.Join(dest, "golden.json")
	if err := os.WriteFile(kept, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := evidence.Publish(dest, []string{"golden"}, []evidence.Subject{sampleSubject(t, "golden", 1)}); err == nil {
		t.Fatal("an existing destination was overwritten")
	}
	if raw, _ := os.ReadFile(kept); string(raw) != "previous" {
		t.Fatalf("the previous evidence changed: %q", raw)
	}
}

func TestPublishIndexesEveryPublishedSubjectAndNoSkippedOne(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "0.1.0")
	golden := sampleSubject(t, "golden", 2)
	dist := evidence.Subject{Header: evidence.Header{Schema: evidence.Schema, Release: "0.1.0", Subject: "dist"}, Skipped: "DMPF_KAFKA_BROKERS"}

	index, err := evidence.Publish(dest, []string{"dist", "golden"}, []evidence.Subject{golden, dist})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dest, "golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []evidence.IndexEntry{{Subject: "golden", SHA256: evidence.Digest(raw)}}
	if !slices.Equal(index, want) {
		t.Fatalf("index = %+v, want %+v", index, want)
	}
	onDisk, err := os.ReadFile(filepath.Join(dest, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var written []evidence.IndexEntry
	if err := json.Unmarshal(onDisk, &written); err != nil || !slices.Equal(written, want) {
		t.Fatalf("index.json = %s (err = %v), want %+v", onDisk, err, want)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if !slices.Equal(names, []string{"dist.json", "golden.json", "index.json"}) {
		t.Fatalf("published files = %v", names)
	}
}

func TestReadRecordsCollectsTheSubjectsRecordsByName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(evidence.DirEnv, dir)
	evidence.RecordVerdict(t, "services", "accepted", serviceskit.Verdict{})
	evidence.RecordVerdict(t, "services", "rejected", serviceskit.Verdict{})

	records, err := evidence.ReadRecords(dir, "services")
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(records)); !slices.Equal(got, []string{"accepted", "rejected"}) {
		t.Fatalf("records = %v", got)
	}
	if _, err := evidence.ReadRecords(dir, "absent"); err == nil {
		t.Fatal("a subject without records was read as empty")
	}
}
