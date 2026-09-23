package audit

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Identity names the process in every line of the trail. It holds no tenant:
// a process serves every tenant it authenticates, so the line takes the tenant
// of the call from the carrier (IDN-20).
type Identity struct {
	Service  string
	Version  string
	Instance string
}

// envelopeRecord is one line of the audit trail in the envelope the log handler
// uses plus kind, so both channels land in Loki with one shape (LOG-13, DAT-25).
type envelopeRecord struct {
	Time          string `json:"time"`
	Level         string `json:"level"`
	Msg           string `json:"msg"`
	Kind          string `json:"kind"`
	TraceID       string `json:"trace_id"`
	SpanID        string `json:"span_id"`
	Service       string `json:"service"`
	Version       string `json:"version"`
	Instance      string `json:"instance"`
	CorrelationID string `json:"correlation_id"`
	TenantID      string `json:"tenant_id"`
	Subject       string `json:"subject"`
	Object        string `json:"object"`
	Action        string `json:"action"`
	Outcome       string `json:"outcome"`
	DataTenantID  string `json:"data_tenant_id,omitempty"`
}

// NewEnvelopeSink writes the trail to w in the log envelope, one JSON object
// per line, serialising writers so a concurrent trail stays parseable.
//
// It differs from NewJSONSink, which writes the bare Event: this one carries
// the process identity and the trace and correlation of the call, which is
// what a reader needs to join the trail to the logs beside it.
func NewEnvelopeSink(w io.Writer, identity Identity) Sink {
	return &envelopeSink{encoder: json.NewEncoder(w), identity: identity}
}

type envelopeSink struct {
	mu       sync.Mutex
	encoder  *json.Encoder
	identity Identity
}

func (s *envelopeSink) Emit(ctx context.Context, event Event) error {
	record := envelopeRecord{
		Time:     time.Unix(0, int64(event.At)).UTC().Format(time.RFC3339Nano),
		Level:    slog.LevelInfo.String(),
		Msg:      "audit",
		Kind:     "audit",
		Service:  s.identity.Service,
		Version:  s.identity.Version,
		Instance: s.identity.Instance,
		Subject:  event.Subject,
		Object:   event.Object,
		Action:   event.Action,
		Outcome:  event.Outcome,
	}
	if event.Tenant != "" {
		record.TenantID, record.DataTenantID = event.Tenant, event.DataTenant
	} else if execution, ok := ports.ExecutionContextFrom(ctx); ok {
		if tenant, scoped := execution.Tenant(); scoped {
			record.TenantID = string(tenant)
		}
	}
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		record.TraceID, record.SpanID = span.TraceID().String(), span.SpanID().String()
	}
	if mc, ok := ports.MessageContextFrom(ctx); ok {
		record.CorrelationID = mc.CorrelationID
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.encoder.Encode(record)
}
