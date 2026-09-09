package dmpfreference

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/apps/backend/dmpf-reference/api"
	dmpfapplication "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/audit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/logging"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/otelboot"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/otelboot/otlp"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/usecase"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// NewTelemetry boots the one OpenTelemetry runtime of the process (otelboot
// starts once per process). With DMPF_OTLP_ENDPOINT it exports over OTLP/gRPC;
// without it the pipeline stays in memory, which is the development mode.
func NewTelemetry(ctx context.Context, cfg Config, out io.Writer) (*otelboot.Runtime, error) {
	logger := slog.New(logging.NewHandler(out, logging.Config{
		Service:  cfg.Service,
		Version:  cfg.Version,
		Instance: cfg.Instance,
		Class:    tracing.ClassWrite,
		Fields:   requestFields,
	}))

	// The SDK reports export failures through a global handler that defaults
	// to the standard log package — a bare line with no level and no service.
	// Routing it through the platform handler keeps one record shape in Loki.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.WarnContext(ctx, "telemetry export failed", "error", err.Error())
	}))

	config := otelboot.Config{
		// W3C declarado: Trace Context leva traceparent e tracestate — os dois
		// campos que otelboot exige (ErrPropagatorNotW3C) — e Baggage completa
		// o par de recomendações W3C do OpenTelemetry.
		Propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		Resource: otelboot.Resource{
			ServiceName:       cfg.Service,
			ServiceVersion:    cfg.Version,
			ServiceInstanceID: cfg.Instance,
		},
		Logger: logger,
	}

	if cfg.OTLPEndpoint == "" {
		logger.WarnContext(ctx, "telemetry kept in memory (development mode): DMPF_OTLP_ENDPOINT is unset")
		config.TraceExporter = tracetest.NewInMemoryExporter()
		config.MetricReader = sdkmetric.NewManualReader()
		return otelboot.Start(ctx, config)
	}

	if cfg.OTLPInsecure {
		logger.WarnContext(ctx, "telemetry exported without TLS: DMPF_OTLP_INSECURE is set (development and CI only)", "endpoint", cfg.OTLPEndpoint)
	}
	config.Transport = otelboot.Transport{Endpoint: cfg.OTLPEndpoint, Insecure: cfg.OTLPInsecure}
	config.AllowInsecure = cfg.OTLPInsecure
	exporter, err := otlp.TraceExporter(ctx, config)
	if err != nil {
		return nil, err
	}
	reader, err := otlp.MetricReader(ctx, config)
	if err != nil {
		return nil, err
	}
	config.TraceExporter, config.MetricReader = exporter, reader
	return otelboot.Start(ctx, config)
}

// requestFields is what the log handler stamps on every record written inside
// a request (LOG-01): the correlation the edge authored and the single tenant
// this edge knows. A record written outside a request has neither, and the
// handler prints them absent — a startup line has no chain to belong to.
func requestFields(ctx context.Context) logging.Fields {
	fields := logging.Fields{logging.KeyTenantID: api.Tenant}
	if mc, ok := dmpfports.MessageContextFrom(ctx); ok {
		fields[logging.KeyCorrelationID] = mc.CorrelationID
	}
	return fields
}

// auditRecord is one line of the audit trail in the envelope the log handler
// uses — time, level, msg, service, version, instance, trace and correlation —
// plus kind, so the two channels land in Loki with one shape and a detectable
// level while the trail stays its own writer, never sampled (LOG-13, DAT-25).
type auditRecord struct {
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
}

type auditSink struct {
	mu      sync.Mutex
	encoder *json.Encoder
	cfg     Config
}

// NewAuditSink writes the trail to w, one JSON object per line, serialising
// writers so a concurrent trail stays parseable.
func NewAuditSink(w io.Writer, cfg Config) audit.Sink {
	return &auditSink{encoder: json.NewEncoder(w), cfg: cfg}
}

func (s *auditSink) Emit(ctx context.Context, event audit.Event) error {
	record := auditRecord{
		Time:     time.Unix(0, int64(event.At)).UTC().Format(time.RFC3339Nano),
		Level:    slog.LevelInfo.String(),
		Msg:      "audit",
		Kind:     "audit",
		Service:  s.cfg.Service,
		Version:  s.cfg.Version,
		Instance: s.cfg.Instance,
		TenantID: api.Tenant,
		Subject:  event.Subject,
		Object:   event.Object,
		Action:   event.Action,
		Outcome:  event.Outcome,
	}
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		record.TraceID, record.SpanID = span.TraceID().String(), span.SpanID().String()
	}
	if mc, ok := dmpfports.MessageContextFrom(ctx); ok {
		record.CorrelationID = mc.CorrelationID
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.encoder.Encode(record)
}

// classify hands the instrumentation the category of FND-07 when the failure
// carries one; anything else is counted as unclassified, never by its message.
func classify(err error) string {
	if failure, ok := errors.AsType[*dmpfapplication.Failure](err); ok {
		return string(failure.Category())
	}
	return usecase.CategoryUnclassified
}

// subject is absent by design: the identity of FND-07 has no realization in
// the kernel and this edge authenticates nobody, so the audit records it as such.
func subject(context.Context) string { return "" }
