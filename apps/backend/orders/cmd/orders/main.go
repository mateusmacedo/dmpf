// Command orders runs one role of the orders context: the gRPC
// api or the relay, selected by --role and configured only through the
// environment.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/mateusmacedo/dmpf/apps/backend/orders"
)

// A refused start (usage or configuration) must never read as a failed run to
// whoever only checks "non-zero".
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	role := flag.String("role", "", "process to run: api|relay")
	flag.Parse()

	os.Exit(run(options{role: *role, lookup: os.Getenv}, os.Stdout, os.Stderr))
}

type options struct {
	role   string
	lookup func(string) string
}

func run(o options, out, errOut io.Writer) int {
	if o.role == "" {
		_, _ = fmt.Fprintln(errOut, "use --role api|relay")
		return exitUsage
	}

	cfg, err := orders.FromEnv(orders.Role(o.role), o.lookup)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "orders: %v\n", err)
		return exitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := orders.Run(ctx, cfg, out); err != nil {
		_, _ = fmt.Fprintf(errOut, "orders: %s: %v\n", cfg.Role, err)
		return exitFailure
	}
	return exitOK
}
