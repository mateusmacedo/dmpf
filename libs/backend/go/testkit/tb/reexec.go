package tb

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Reexec runs the test named run alone in a child of the test binary, with env
// added, and answers both streams and the exit: the way to observe an effect on
// the whole process, such as a write to stderr, without suffering it.
func Reexec(t testing.TB, run string, env ...string) (stdout, stderr string, err error) {
	t.Helper()
	ctx := t.Context()
	if deadlined, ok := t.(interface{ Deadline() (time.Time, bool) }); ok {
		if deadline, set := deadlined.Deadline(); set {
			var cancel context.CancelFunc
			ctx, cancel = context.WithDeadline(ctx, deadline)
			defer cancel()
		}
	}
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+run+"$", "-test.count=1")
	command.Env = append(os.Environ(), env...)
	var out, errs bytes.Buffer
	command.Stdout, command.Stderr = &out, &errs
	err = command.Run()
	return out.String(), errs.String(), err
}
