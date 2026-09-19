package boot

import (
	"context"
	"io"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

// Boot starts the telemetry, hands the running runtime to the work and closes
// the pipelines on every exit, including the failing one.
//
// The work is a closure rather than a signature of its own because each
// composition root delegates differently — some pass the writer along, some do
// not — and what is common is only the order: no work without telemetry, and
// no exit without the drain.
func Boot(ctx context.Context, out io.Writer, t Telemetry, work func(context.Context, *otelboot.Runtime) error) error {
	rt, err := StartTelemetry(ctx, out, t)
	if err != nil {
		return err
	}
	defer rt.ShutdownGracefully(ctx)
	return work(ctx, rt)
}
