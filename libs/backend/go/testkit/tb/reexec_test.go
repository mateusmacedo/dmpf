package tb_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const reexecChildEnv = "TB_REEXEC_CHILD"

func TestReexecChild(t *testing.T) {
	switch os.Getenv(reexecChildEnv) {
	case "speak":
		_, _ = fmt.Fprint(os.Stdout, "to stdout")
		_, _ = fmt.Fprint(os.Stderr, "to stderr")
	case "fail":
		os.Exit(3)
	case "hang":
		time.Sleep(time.Minute)
	default:
		t.Skip("runs only as the process the Reexec tests start")
	}
}

func TestReexecAnswersWhatTheChildWrote(t *testing.T) {
	stdout, stderr, err := tb.Reexec(t, "TestReexecChild", reexecChildEnv+"=speak")

	if err != nil {
		t.Fatalf("Reexec() = %v, want nil", err)
	}
	if !strings.Contains(stdout, "to stdout") || stderr != "to stderr" {
		t.Fatalf("stdout = %q, stderr = %q, want each stream apart", stdout, stderr)
	}
}

func TestReexecAnswersTheExitOfTheChild(t *testing.T) {
	_, _, err := tb.Reexec(t, "TestReexecChild", reexecChildEnv+"=fail")

	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("Reexec() = %v, want the exit status 3 of the child", err)
	}
}

type deadlineTB struct {
	testing.TB
	deadline time.Time
}

func (d deadlineTB) Deadline() (time.Time, bool) { return d.deadline, true }

func TestReexecStopsTheChildAtTheDeadlineOfTheTest(t *testing.T) {
	start := time.Now()

	_, _, err := tb.Reexec(deadlineTB{TB: t, deadline: start.Add(500 * time.Millisecond)}, "TestReexecChild", reexecChildEnv+"=hang")

	if err == nil {
		t.Fatal("Reexec() = nil, want the child killed at the deadline")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("Reexec() returned after %v, want it bounded by the deadline", elapsed)
	}
}
