package evidence

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strings"
)

type TestResult struct {
	Package string `json:"package"`
	Test    string `json:"test,omitempty"`
	Action  string `json:"action"`
}

type testEvent struct {
	Action  string
	Package string
	Test    string
}

// FilterStream keeps only the terminal actions of a go test -json stream: Time,
// Elapsed and Output change between two runs over the same commit, and a digest
// over them would never reproduce.
func FilterStream(r io.Reader) ([]TestResult, error) {
	var out []TestResult
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for line := 1; sc.Scan(); line++ {
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		var ev testEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			return nil, fmt.Errorf("go test -json line %d is not an event: %w", line, err)
		}
		switch ev.Action {
		case "pass", "fail", "skip":
			out = append(out, TestResult{Package: ev.Package, Test: ev.Test, Action: ev.Action})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("go test -json: %w", err)
	}
	slices.SortFunc(out, func(a, b TestResult) int {
		return cmp.Or(cmp.Compare(a.Package, b.Package), cmp.Compare(a.Test, b.Test), cmp.Compare(a.Action, b.Action))
	})
	return out, nil
}

func JudgeResults(results []TestResult, ci bool) []string {
	var failures []string
	for _, r := range results {
		switch {
		case r.Action == "fail":
			failures = append(failures, "fail: "+describe(r))
		case r.Action == "skip" && ci:
			failures = append(failures, "skip under CI: "+describe(r))
		}
	}
	return failures
}

func describe(r TestResult) string {
	if r.Test == "" {
		return r.Package
	}
	return r.Package + " " + r.Test
}

type GoTest struct {
	Dir      string
	Tags     []string
	Packages []string
	Exclude  []string
	Skip     []string
	Env      []string
}

// RunGoTest does not treat a non-zero exit as an error: a failing test is a
// result to judge. It errors only when the stream carries no result at all.
func RunGoTest(ctx context.Context, spec GoTest) ([]TestResult, error) {
	args := []string{"test", "-json", "-count=1", "-p", "1"}
	if len(spec.Tags) > 0 {
		args = append(args, "-tags="+strings.Join(spec.Tags, ","))
	}
	if len(spec.Skip) > 0 {
		quoted := make([]string, len(spec.Skip))
		for i, name := range spec.Skip {
			quoted[i] = regexp.QuoteMeta(name)
		}
		args = append(args, "-skip=^("+strings.Join(quoted, "|")+")$")
	}
	packages, err := included(ctx, spec)
	if err != nil {
		return nil, err
	}
	args = append(args, packages...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	results, err := FilterStream(&stdout)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("go %s: no test result (%v): %s", strings.Join(args, " "), runErr, strings.TrimSpace(stderr.String()))
	}
	return results, nil
}

func included(ctx context.Context, spec GoTest) ([]string, error) {
	if len(spec.Exclude) == 0 {
		return spec.Packages, nil
	}
	args := []string{"list", "-f", "{{.ImportPath}}"}
	if len(spec.Tags) > 0 {
		args = append(args, "-tags="+strings.Join(spec.Tags, ","))
	}
	cmd := exec.CommandContext(ctx, "go", append(args, spec.Packages...)...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	var kept []string
	for _, pkg := range strings.Fields(string(out)) {
		if !slices.ContainsFunc(spec.Exclude, func(pattern string) bool { return excluded(pattern, pkg) }) {
			kept = append(kept, pkg)
		}
	}
	return kept, nil
}

func excluded(pattern, pkg string) bool {
	base, below := strings.CutSuffix(pattern, "/...")
	segments := strings.Split(pkg, "/")
	for n := len(segments); n > 0; n-- {
		if matched, _ := path.Match(base, strings.Join(segments[:n], "/")); matched {
			return below || n == len(segments)
		}
	}
	return false
}
