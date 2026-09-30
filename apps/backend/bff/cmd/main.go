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
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app"
)

// A refused start must never read as a failed run to whoever only checks
// "non-zero".
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

const (
	healthcheckCommand = "healthcheck"
	probeTimeout       = 3 * time.Second
)

func main() {
	os.Exit(run(options{lookup: os.Getenv, args: os.Args[1:]}, os.Stdout, os.Stderr))
}

type options struct {
	lookup func(string) string
	args   []string
}

func run(o options, out, errOut io.Writer) int {
	if len(o.args) > 0 && o.args[0] == healthcheckCommand {
		return healthcheck(o.lookup, errOut)
	}
	cfg, err := app.FromEnv(o.lookup)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "bff: %v\n", err)
		return exitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := app.Run(ctx, cfg, out); err != nil {
		_, _ = fmt.Fprintf(errOut, "bff: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// WHY: Docker's HEALTHCHECK reads only 0 and 1 and reserves 2, so a
// configuration the edge could not start with is a failure, not a usage error.
func healthcheck(lookup func(string) string, errOut io.Writer) int {
	cfg, err := app.FromEnv(lookup)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "bff healthcheck: %v\n", err)
		return exitFailure
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	if err := app.Probe(ctx, cfg); err != nil {
		_, _ = fmt.Fprintf(errOut, "bff healthcheck: %v\n", err)
		return exitFailure
	}
	return exitOK
}
