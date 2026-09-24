package otelboot_test

import (
	"context"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
)

func TestShutdownGracefullyClosesThePipelinesOfACancelledProcess(t *testing.T) {
	rt, _ := startedRuntime(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rt.ShutdownGracefully(ctx)

	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() after ShutdownGracefully() = %v, want nil: the shutdown is idempotent and already ran", err)
	}
}

func TestShutdownGracefullyReturnsWithinTheGraceWindow(t *testing.T) {
	rt, _ := startedRuntime(t, nil)

	start := time.Now()
	rt.ShutdownGracefully(context.Background())

	if elapsed := time.Since(start); elapsed > observability.ShutdownGrace {
		t.Fatalf("ShutdownGracefully() took %v, want at most the grace window of %v", elapsed, observability.ShutdownGrace)
	}
}
