//go:build integration && distributed

package distkit

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
)

// RunRole is the body a test package gives its TestDistkitRole: it reads the
// role before anything else and skips when there is none, so the function is
// inert in every run that is not a re-executed child of the harness.
func RunRole(t *testing.T) {
	t.Helper()
	role := Role(os.Getenv(EnvRole))
	if role == "" {
		t.Skip(EnvRole + " unset: this test only runs as a re-executed child of distkit.Harness")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	switch role {
	case RoleRelay:
		drain(t, ctx)
	default:
		t.Fatalf("distkit: unknown role %q", role)
	}
}

// drain runs the real relay of this context, entering by FromEnv like the
// binary does, so what the harness judges is the production path and a
// configuration the harness cannot accidentally fill in by hand.
func drain(t *testing.T, ctx context.Context) {
	t.Helper()
	cfg, err := app.FromEnv(app.RoleRelay, os.Getenv)
	if err != nil {
		t.Fatalf("distkit: the drain refused its configuration: %v", err)
	}
	if err := app.Run(ctx, cfg, os.Stdout); err != nil && !isShutdown(err) {
		t.Fatalf("distkit: the drain returned %v", err)
	}
}

// isShutdown reads the exit a cancelled context produces, which is how the
// parent asks the child to finish and not a failure of the drain.
func isShutdown(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
