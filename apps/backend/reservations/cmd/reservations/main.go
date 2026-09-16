// Command reservations runs one role of the reservations
// context: the gRPC api, the relay or the consumer, selected by --role and
// configured only through the environment.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations"
)

// A refused start (usage or configuration) must never read as a failed run to
// whoever only checks "non-zero".
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	role := flag.String("role", "", "process to run: api|relay|consumer")
	flag.Parse()

	os.Exit(run(options{role: *role, lookup: os.Getenv}, os.Stdout, os.Stderr))
}

type options struct {
	role   string
	lookup func(string) string
}

func run(o options, out, errOut io.Writer) int {
	if o.role == "" {
		_, _ = fmt.Fprintln(errOut, "use --role api|relay|consumer")
		return exitUsage
	}

	cfg, err := reservations.FromEnv(reservations.Role(o.role), o.lookup)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "reservations: %v\n", err)
		return exitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := reservations.Run(ctx, cfg, out); err != nil {
		_, _ = fmt.Fprintf(errOut, "reservations: %s: %v\n", cfg.Role, err)
		return exitFailure
	}
	return exitOK
}
