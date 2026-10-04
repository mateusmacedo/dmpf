package otelboot

import (
	"context"
	"reflect"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

// ShutdownGracefully closes the pipelines within the grace window and reports
// a trace or metric failure instead of returning it, so it can be deferred at
// the top of a composition root.
//
// WHY: context.WithoutCancel drops the cancellation and adds no deadline, and
// Shutdown hands the context straight to both providers, so an unreachable
// collector blocks the exit until the orchestrator sends SIGKILL.
func (r *Runtime) ShutdownGracefully(ctx context.Context) {
	grace, cancel := context.WithTimeout(context.WithoutCancel(ctx), observability.ShutdownGrace)
	defer cancel()
	_ = r.shutdown(grace, func(err error) {
		r.LoggerFor(reflect.TypeFor[Runtime]().PkgPath()).WarnContext(grace, "telemetry shutdown", redact.Error(err))
	})
}
