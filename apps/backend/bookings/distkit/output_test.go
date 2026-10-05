//go:build integration && distributed

package distkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOutputIsTheSentinelWhileTheChildRuns(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	cmd := exec.Command("sh", "-c", `trap "exit 0" TERM; echo tick; : > "$1"; while :; do echo tick; sleep 0.01; done`, "sh", ready)
	p := launch(t, Role("output"), cmd)
	awaitFile(t, ready, 5*time.Second)

	if got := p.Output(); got != "(process still running)" {
		t.Fatalf("Output() while the child runs = %q, want the sentinel", got)
	}

	p.Stop(t, 5*time.Second)

	if got := p.Output(); !strings.Contains(got, "tick") {
		t.Fatalf("Output() after Stop = %q, want what the child wrote", got)
	}
}

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
