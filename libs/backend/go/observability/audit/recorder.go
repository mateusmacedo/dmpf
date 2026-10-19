package audit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	ActionPlatformRead    = ports.AuditPlatformRead
	ActionProjectionWrite = ports.AuditProjectionWrite
)

// Fails closed: the access proceeds only once the Sink confirms the record
// (DAT-25); the log is an additional channel, never a substitute.
type Recorder struct {
	sink   Sink
	logger *slog.Logger
	clock  ports.Clock
}

// NewRecorder panics on a nil part: an access across tenants assembled without
// its trail is a composition root error, not a runtime condition.
func NewRecorder(sink Sink, logger *slog.Logger, clock ports.Clock) *Recorder {
	switch {
	case sink == nil:
		panic("audit: NewRecorder needs a Sink")
	case logger == nil:
		panic("audit: NewRecorder needs a logger")
	case clock == nil:
		panic("audit: NewRecorder needs a Clock")
	}
	return &Recorder{sink: sink, logger: logger, clock: clock}
}

// A Sink refusal drops the Sink's message, which may carry what redaction
// exists to keep out (LOG-07).
func (r *Recorder) Record(ctx context.Context, event Event) error {
	if event.At == 0 {
		event.At = r.clock.Now()
	}
	fields := logFields(event)
	if err := r.sink.Emit(ctx, event); err != nil {
		r.logger.ErrorContext(ctx, "dmpf: audit sink refused the record; the access fails closed",
			append(fields, redact.Error(err))...)
		return fmt.Errorf("audit: %s on %s: %w", event.Action, event.Object, ports.ErrAuditUnavailable)
	}
	r.logger.InfoContext(ctx, "dmpf: audit record", fields...)
	return nil
}

// Providers record without importing this package; a nil Recorder gives a nil
// trail, which the provider refuses at construction.
func (r *Recorder) Trail() ports.AuditTrail {
	if r == nil {
		return nil
	}
	return portTrail{recorder: r}
}

type portTrail struct{ recorder *Recorder }

func (t portTrail) Record(ctx context.Context, record ports.AuditRecord) error {
	return t.recorder.Record(ctx, Event{
		Subject:    record.Subject,
		Object:     record.Object,
		Action:     record.Action,
		Outcome:    record.Outcome,
		Tenant:     record.Tenant,
		DataTenant: record.DataTenant,
	})
}

func logFields(event Event) []any {
	kept := attributes(event)
	fields := make([]any, 0, len(kept)+1)
	for _, kv := range kept {
		fields = append(fields, slog.String(string(kv.Key), kv.Value.AsString()))
	}
	return fields
}
