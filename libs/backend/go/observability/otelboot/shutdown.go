package otelboot

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
)

// ShutdownGracefully closes the pipelines within the grace window and reports
// a failure instead of returning it, so it can be deferred at the top of a
// composition root.
//
// WHY: context.WithoutCancel drops the cancellation and adds no deadline, and
// Shutdown hands the context straight to both providers, so an unreachable
// collector blocks the exit until the orchestrator sends SIGKILL.
func (r *Runtime) ShutdownGracefully(ctx context.Context) {
	grace, cancel := context.WithTimeout(context.WithoutCancel(ctx), observability.ShutdownGrace)
	defer cancel()
	if err := r.Shutdown(grace); err != nil {
		r.Logger().WarnContext(grace, "telemetry shutdown", "error", err.Error())
	}
}
