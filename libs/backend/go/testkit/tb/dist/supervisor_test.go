package dist_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
)

func awaitFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s did not appear within %v", path, timeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestOutputReadsWhatTheChildWroteWhileItRuns(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	p := dist.Supervise(t, "ticker", exec.Command("sh", "-c",
		`trap "exit 0" TERM; echo tick; : > "$1"; while :; do echo tick; sleep 0.01; done`, "sh", ready))
	awaitFile(t, ready, 5*time.Second)

	if got := p.Output(); !strings.Contains(got, "tick") {
		t.Fatalf("Output() while the child runs = %q, want what it wrote so far", got)
	}
	p.Stop(t, 5*time.Second)
	select {
	case <-p.Done():
	default:
		t.Fatal("Done() still open after Stop")
	}
	if p.Err() != nil || p.Name() != "ticker" {
		t.Fatalf("Err(), Name() = %v, %q; want a clean exit of ticker", p.Err(), p.Name())
	}
}

func TestErrReportsTheExitOfTheChild(t *testing.T) {
	p := dist.Supervise(t, "failing", exec.Command("sh", "-c", "echo broken; exit 3"))
	<-p.Done()

	if p.Err() == nil || !strings.Contains(p.Output(), "broken") {
		t.Fatalf("Err(), Output() = %v, %q; want the exit status and what the child wrote", p.Err(), p.Output())
	}
}

func TestTheCleanupTerminatesAChildStillRunning(t *testing.T) {
	var p *dist.Process
	t.Run("supervised", func(t *testing.T) {
		p = dist.Supervise(t, "sleeper", exec.Command("sleep", "30"))
	})

	select {
	case <-p.Done():
	case <-time.After(30 * time.Second):
		t.Fatal("the child outlived the test that supervised it")
	}
}
