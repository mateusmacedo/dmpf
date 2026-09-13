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
	Env      []string
}

// RunGoTest does not treat a non-zero exit as an error: a failing test is a
// result to judge. It errors only when the stream carries no result at all.
func RunGoTest(ctx context.Context, spec GoTest) ([]TestResult, error) {
	args := []string{"test", "-json", "-count=1", "-p", "1"}
	if len(spec.Tags) > 0 {
		args = append(args, "-tags="+strings.Join(spec.Tags, ","))
	}
	args = append(args, spec.Packages...)
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
