package dist_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
)

func TestOutputReadsWhatTheChildWroteWhileItRuns(t *testing.T) {
	p := dist.Supervise(t, "ticker", exec.Command("sh", "-c",
		`trap "exit 0" TERM; while :; do echo tick; sleep 0.01; done`))
	dist.WaitFor(t, "the child to write a tick", 5*time.Second, func() bool {
		return strings.Contains(p.Output(), "tick")
	})

	select {
	case <-p.Done():
		t.Fatalf("the child exited before Stop: %v", p.Err())
	default:
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

func TestOutputKeepsOnlyTheEndOfALongOutput(t *testing.T) {
	p := dist.Supervise(t, "flood", exec.Command("sh", "-c", `yes flood | head -c 3000000; echo end`))
	<-p.Done()

	got := p.Output()
	if len(got) > 1<<20 || !strings.HasSuffix(got, "end\n") {
		t.Fatalf("Output() holds %d bytes ending in %q, want at most 1 MiB ending in what the child wrote last", len(got), got[max(0, len(got)-16):])
	}
}

func TestTheCleanupTerminatesAChildStillRunning(t *testing.T) {
	var p *dist.Process
	t.Run("supervised", func(t *testing.T) {
		p = dist.Supervise(t, "sleeper", exec.Command("sleep", "300"))
	})

	select {
	case <-p.Done():
	case <-time.After(30 * time.Second):
		t.Fatal("the child outlived the test that supervised it")
	}
}
