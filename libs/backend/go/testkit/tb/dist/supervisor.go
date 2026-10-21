package dist

import (
	"bytes"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	stopGrace = 20 * time.Second
	waitDelay = 5 * time.Second

	// A child logging in a tight loop would grow the parent without bound; the
	// end of the output is what explains a failure, so that is what stays.
	outputCap = 1 << 20
)

type output struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buf.Write(p)
	if excess := o.buf.Len() - outputCap; excess > 0 {
		o.buf.Next(excess)
	}
	return len(p), nil
}

func (o *output) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

type Process struct {
	name     string
	cmd      *exec.Cmd
	output   output
	finished chan struct{}
	err      error
}

// Supervise starts cmd and ends it with the test: SIGTERM, a grace period and a
// kill, without failing the test, so a child that misbehaves on shutdown is
// reported by whoever waits on it, not by the cleanup.
func Supervise(t testing.TB, name string, cmd *exec.Cmd) *Process {
	t.Helper()
	p := &Process{name: name, cmd: cmd, finished: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = &p.output, &p.output
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = waitDelay
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("dist: start %s: %v", name, err)
	}
	go func() {
		p.err = cmd.Wait()
		close(p.finished)
	}()
	t.Cleanup(func() { p.terminate(stopGrace) })
	return p
}

func (p *Process) Name() string { return p.name }

func (p *Process) Done() <-chan struct{} { return p.finished }

// Err is the exit of the child, meaningful only once Done is closed.
func (p *Process) Err() error { return p.err }

func (p *Process) Output() string { return p.output.String() }

func (p *Process) terminate(grace time.Duration) {
	select {
	case <-p.finished:
		return
	default:
	}
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-p.finished:
	case <-time.After(grace):
		_ = p.cmd.Process.Kill()
		<-p.finished
	}
}

// Kill is a crash, not a stop: SIGKILL leaves the child no deferred call nor
// shutdown path.
func (p *Process) Kill() {
	_ = p.cmd.Process.Kill()
	<-p.finished
}

func (p *Process) Wait(t testing.TB, timeout time.Duration) {
	t.Helper()
	select {
	case <-p.finished:
		if p.err != nil {
			t.Fatalf("dist: %s exited with %v\n%s", p.name, p.err, p.Output())
		}
	case <-time.After(timeout):
		_ = p.cmd.Process.Kill()
		<-p.finished
		t.Fatalf("dist: %s did not exit within %v\n%s", p.name, timeout, p.Output())
	}
}

func (p *Process) Stop(t testing.TB, timeout time.Duration) {
	t.Helper()
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	p.Wait(t, timeout)
}
