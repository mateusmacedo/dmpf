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

// reservedKeys are the keys the platform writes at the root of every record.
// They are reserved because JSON tolerates a duplicated key and readers keep
// the last one: an author attribute named service would silently take the place
// of the service the record is about (LOG-01).
var reservedKeys = []string{
	KeyTraceID, KeySpanID, KeyService, KeyVersion, KeyInstance,
	KeyCorrelationID, KeyRequestID, KeyTenantID,
}

// AuthorPrefix renames what the author put on a reserved key. Renaming keeps
// the author's value in the record, which dropping it would not, and still
// leaves the reserved key to the platform.
const AuthorPrefix = "app."

func isReserved(key string) bool { return slices.Contains(reservedKeys, key) }

// rename moves a reserved key out of the way, recursively for a group, so a
// nested attribute cannot re-enter through it.
func rename(attr slog.Attr) slog.Attr {
	if attr.Value.Kind() == slog.KindGroup && isReserved(attr.Key) {
		return slog.Attr{Key: AuthorPrefix + attr.Key, Value: attr.Value}
	}
	if isReserved(attr.Key) {
		return slog.Attr{Key: AuthorPrefix + attr.Key, Value: attr.Value}
	}
	return attr
}

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
	steps    []step
	grouped  bool
	config   Config
	sampler  sampler
	redactor redact.Redactor
	warnOnce *sync.Once
	clashes  *sync.Once
}

// step is one call the author made. The group is tracked apart from the
// attributes because only the root of the record has reserved keys: inside an
// author group, service is payload.service and collides with nothing.
type step struct {
	group string
	attrs []slog.Attr
}

func (s step) apply(inner slog.Handler) slog.Handler {
	if s.group != "" {
		return inner.WithGroup(s.group)
	}
	return inner.WithAttrs(s.attrs)
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
		clashes:  &sync.Once{},
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
	return h.effective(ctx).Handle(ctx, h.guard(ctx, record))
}

// guard renames the attributes of the record that would land on a reserved key.
// It only acts at the root: under an author group the keys are already prefixed
// by the group and cannot collide.
func (h *handler) guard(ctx context.Context, record slog.Record) slog.Record {
	if h.grouped {
		return record
	}

	clashing := make([]string, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		if isReserved(attr.Key) {
			clashing = append(clashing, attr.Key)
		}
		return true
	})
	if len(clashing) == 0 {
		return record
	}

	h.reportClashes(ctx, clashing)

	guarded := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		guarded.AddAttrs(rename(attr))
		return true
	})
	return guarded
}

// effective is the base handler carrying the mandatory fields, with the
// author's steps replayed on top of them.
func (h *handler) effective(ctx context.Context) slog.Handler {
	built := h.base.WithAttrs(h.mandatory(ctx))
	for _, applied := range h.steps {
		built = applied.apply(built)
	}
	return built
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	kept := attrs
	if !h.grouped {
		clashing := make([]string, 0, len(attrs))
		kept = make([]slog.Attr, 0, len(attrs))
		for _, attr := range attrs {
			if isReserved(attr.Key) {
				clashing = append(clashing, attr.Key)
			}
			kept = append(kept, rename(attr))
		}
		if len(clashing) > 0 {
			h.reportClashes(context.Background(), clashing)
		}
	}

	next := *h
	next.steps = append(slices.Clone(h.steps), step{attrs: kept})
	return &next
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	opened := name
	if !h.grouped && isReserved(name) {
		h.reportClashes(context.Background(), []string{name})
		opened = AuthorPrefix + name
	}

	next := *h
	next.steps = append(slices.Clone(h.steps), step{group: opened})
	next.grouped = true
	return &next
}

// reportClashes warns once per handler that reserved keys were renamed. One
// warning per record would turn a single bad call site into a flood.
func (h *handler) reportClashes(ctx context.Context, clashing []string) {
	h.clashes.Do(func() {
		if !h.base.Enabled(ctx, slog.LevelWarn) {
			return
		}

		slices.Sort(clashing)
		warning := slog.NewRecord(time.Now(), slog.LevelWarn,
			"dmpf: keys reserved by the platform were renamed to keep the mandatory fields", 0)
		warning.AddAttrs(
			slog.String(KeyService, h.config.Service),
			slog.Any("renamed_keys", slices.Compact(clashing)),
			slog.String("prefix", AuthorPrefix),
		)
		_ = h.base.Handle(ctx, warning)
	})
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
