package audit

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

// EventName and Scope tell the trail apart from the logs of the process: the
// operational panels exclude it by scope_name (RF-A7).
const (
	EventName = "dmpf.audit"
	Scope     = "github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
)

const (
	KeySubject      = "dmpf.audit.subject"
	KeyObject       = "dmpf.audit.object"
	KeyAction       = "dmpf.audit.action"
	KeyOutcome      = "dmpf.audit.outcome"
	KeyDataTenantID = "dmpf.audit.data_tenant_id"
)

const message = "audit"

// NewLogSink emits the trail through the Logs API of provider. The correlation
// of the call comes from the baggage processor of the provider and the trace
// from ctx; nothing of the event is written to the span (DAT-25).
func NewLogSink(provider log.LoggerProvider) Sink {
	return logSink{logger: provider.Logger(Scope)}
}

type logSink struct {
	logger log.Logger
}

func (s logSink) Emit(ctx context.Context, event Event) error {
	var record log.Record
	record.SetEventName(EventName)
	if event.At != 0 {
		record.SetTimestamp(time.Unix(0, int64(event.At)))
	}
	record.SetSeverity(log.SeverityInfo)
	record.SetSeverityText(log.SeverityInfo.String())
	record.SetBody(attribute.StringValue(message))
	record.AddAttributes(attributes(event)...)
	s.logger.Emit(ctx, record)
	return nil
}

func attributes(event Event) []attribute.KeyValue {
	values := []struct{ key, value string }{
		{KeySubject, event.Subject},
		{KeyObject, event.Object},
		{KeyAction, event.Action},
		{KeyOutcome, event.Outcome},
		{tracing.KeyTenantID, event.Tenant},
		{KeyDataTenantID, event.DataTenant},
	}
	kept := make([]attribute.KeyValue, 0, len(values))
	for _, v := range values {
		if v.value != "" {
			kept = append(kept, attribute.String(v.key, v.value))
		}
	}
	return kept
}
