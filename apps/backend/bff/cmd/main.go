// Command bff runs the public REST edge of the reference
// topology, configured only through the environment.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/mateusmacedo/dmpf/apps/backend/bff"
)

// A refused start must never read as a failed run to whoever only checks
// "non-zero".
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run(options{lookup: os.Getenv}, os.Stdout, os.Stderr))
}

type options struct {
	lookup func(string) string
}

func run(o options, out, errOut io.Writer) int {
	cfg, err := bff.FromEnv(o.lookup)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "bff: %v\n", err)
		return exitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := bff.Run(ctx, cfg, out); err != nil {
		_, _ = fmt.Fprintf(errOut, "bff: %v\n", err)
		return exitFailure
	}
	return exitOK
}
