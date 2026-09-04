package logging

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/redact"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
)

// Keys of the mandatory fields of LOG-01 and LOG-04.
const (
	KeyTraceID       = "trace_id"
	KeySpanID        = "span_id"
	KeyService       = "service"
	KeyVersion       = "version"
	KeyInstance      = "instance"
	KeyCorrelationID = "correlation_id"
	KeyRequestID     = "request_id"
	KeyTenantID      = "tenant_id"
)

// correlationKeys are the only keys the extractor may contribute. Anything else
// is ignored: the field set of a record is a contract, not a free-form map.
var correlationKeys = []string{KeyCorrelationID, KeyRequestID, KeyTenantID}

// Fields is what the edge extracts from the context. It is a map and not a
// struct because the extractor is injected and may return whatever it likes;
// the handler is what enforces the allowlist.
type Fields map[string]string

// Config declares the record shape of the service.
type Config struct {
	Service  string
	Version  string
	Instance string

	// Fields extracts the correlation of the request. It is injected by the
	// edge because trace.Span does not expose attributes already recorded and
	// the kernel admits no context value beyond the retry budget. A nil
	// extractor records the three fields as absent.
	Fields func(ctx context.Context) Fields

	// AllowedFields is the redaction allowlist of the service.
	AllowedFields []string

	// Class is the traffic class of this service, which drives the sampling
	// rate of records below error.
	Class tracing.Class

	// Sampling overrides the platform rates of TRC-13.
	Sampling tracing.Rates

	// Level is the minimum severity. A nil level means slog's default.
	Level slog.Leveler

	// Rand draws the sampling decision. It is injected so a test fixes the draw
	// instead of tolerating a range.
	Rand func() float64
}

// handler wraps a JSON handler and injects the mandatory fields, so the author
// of a record never passes correlation by hand and cannot forget it.
//
// The author's WithAttrs and WithGroup are kept as steps replayed on each
// record, and not applied to the inner handler once: the mandatory fields must
// precede them, or a WithGroup would nest service and trace_id inside the
// author's group and a query for them would stop finding them (LOG-01).
type handler struct {
	base     slog.Handler
	steps    []func(slog.Handler) slog.Handler
	config   Config
	sampler  sampler
	redactor redact.Redactor
	warnOnce *sync.Once
}

// NewHandler builds the platform handler over w.
func NewHandler(w io.Writer, config Config) slog.Handler {
	rates := config.Sampling
	if rates == nil {
		rates = tracing.DefaultRates()
	}
	class := config.Class
	if class == "" {
		class = tracing.ClassUnclassified
	}

	return &handler{
		base:     slog.NewJSONHandler(w, &slog.HandlerOptions{Level: config.Level}),
		config:   config,
		sampler:  sampler{class: class, rates: rates, rand: config.Rand},
		redactor: redact.New(config.AllowedFields...),
		warnOnce: &sync.Once{},
	}
}

// Redactor is the allowlist of the service, so the author of a record redacts
// with the same list the handler was configured with.
func Redactor(h slog.Handler) (redact.Redactor, bool) {
	platform, ok := h.(*handler)
	if !ok {
		return redact.Redactor{}, false
	}
	return platform.redactor, true
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level) && h.sampler.allows(ctx, level)
}

func (h *handler) Handle(ctx context.Context, record slog.Record) error {
	return h.effective(ctx).Handle(ctx, record)
}

// effective is the base handler carrying the mandatory fields, with the
// author's steps replayed on top of them.
func (h *handler) effective(ctx context.Context) slog.Handler {
	built := h.base.WithAttrs(h.mandatory(ctx))
	for _, step := range h.steps {
		built = step(built)
	}
	return built
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	next := *h
	next.steps = append(slices.Clone(h.steps), func(inner slog.Handler) slog.Handler {
		return inner.WithAttrs(attrs)
	})
	return &next
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	next := *h
	next.steps = append(slices.Clone(h.steps), func(inner slog.Handler) slog.Handler {
		return inner.WithGroup(name)
	})
	return &next
}

func (h *handler) mandatory(ctx context.Context) []slog.Attr {
	span := trace.SpanContextFromContext(ctx)
	traceID, spanID := "", ""
	if span.IsValid() {
		traceID, spanID = span.TraceID().String(), span.SpanID().String()
	}

	correlation := h.correlation(ctx)

	return []slog.Attr{
		slog.String(KeyTraceID, traceID),
		slog.String(KeySpanID, spanID),
		slog.String(KeyService, h.config.Service),
		slog.String(KeyVersion, h.config.Version),
		slog.String(KeyInstance, h.config.Instance),
		slog.String(KeyCorrelationID, correlation[KeyCorrelationID]),
		slog.String(KeyRequestID, correlation[KeyRequestID]),
		slog.String(KeyTenantID, correlation[KeyTenantID]),
	}
}

// correlation reads the extractor and keeps only the permitted keys. A key
// outside the allowlist is dropped with a single warning: one per record would
// turn a misconfigured extractor into a flood.
func (h *handler) correlation(ctx context.Context) Fields {
	kept := Fields{}
	if h.config.Fields == nil {
		return kept
	}

	extracted := h.config.Fields(ctx)
	unexpected := make([]string, 0, len(extracted))
	for key, value := range extracted {
		if slices.Contains(correlationKeys, key) {
			kept[key] = value
			continue
		}
		unexpected = append(unexpected, key)
	}

	if len(unexpected) > 0 {
		slices.Sort(unexpected)
		h.warnOnce.Do(func() { h.warnUnexpected(ctx, unexpected) })
	}
	return kept
}

func (h *handler) warnUnexpected(ctx context.Context, unexpected []string) {
	if !h.base.Enabled(ctx, slog.LevelWarn) {
		return
	}

	warning := slog.NewRecord(time.Now(), slog.LevelWarn,
		"dmpf: the field extractor returned keys outside the allowlist and they were ignored", 0)
	warning.AddAttrs(
		slog.String(KeyService, h.config.Service),
		slog.Any("ignored_keys", unexpected),
		slog.Any("allowed_keys", slices.Clone(correlationKeys)),
	)
	_ = h.base.Handle(ctx, warning)
}
