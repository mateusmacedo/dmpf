package otelboot

import (
	"context"
	"net/url"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

const redacted = "REDACTED"

var peerKeys = map[attribute.Key]bool{
	semconv.ClientAddressKey:      true,
	semconv.NetworkPeerAddressKey: true,
	semconv.NetworkPeerPortKey:    true,
	semconv.UserAgentOriginalKey:  true,
}

var exceptionDetailKeys = map[attribute.Key]bool{
	semconv.ExceptionMessageKey:    true,
	semconv.ExceptionStacktraceKey: true,
}

// NewPrivacyExporter redacts every span before exporter sees it (RF-B3,
// TRC-15). It decorates the exporter and not the span in OnEnd because a
// ReadOnlySpan takes no write.
func NewPrivacyExporter(exporter sdktrace.SpanExporter) sdktrace.SpanExporter {
	return privacyExporter{inner: exporter}
}

type privacyExporter struct {
	inner sdktrace.SpanExporter
}

func (e privacyExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	clean := make([]sdktrace.ReadOnlySpan, len(spans))
	for i, span := range spans {
		clean[i] = redactedSpan{ReadOnlySpan: span}
	}
	return e.inner.ExportSpans(ctx, clean)
}

func (e privacyExporter) Shutdown(ctx context.Context) error { return e.inner.Shutdown(ctx) }

func (e privacyExporter) ForceFlush(ctx context.Context) error {
	if flusher, ok := e.inner.(interface{ ForceFlush(context.Context) error }); ok {
		return flusher.ForceFlush(ctx)
	}
	return nil
}

type redactedSpan struct {
	sdktrace.ReadOnlySpan
}

func (s redactedSpan) Attributes() []attribute.KeyValue {
	original := s.ReadOnlySpan.Attributes()
	var route, template string
	for _, kv := range original {
		switch kv.Key {
		case semconv.HTTPRouteKey:
			route = kv.Value.AsString()
		case semconv.URLTemplateKey:
			template = kv.Value.AsString()
		}
	}

	kept := make([]attribute.KeyValue, 0, len(original))
	for _, kv := range original {
		switch {
		case peerKeys[kv.Key]:
		case kv.Key == semconv.URLPathKey:
			kept = append(kept, semconv.URLPath(orRedacted(route, redacted)))
		case kv.Key == semconv.URLFullKey:
			kept = append(kept, semconv.URLFull(withoutDetail(kv.Value.AsString(), template)))
		default:
			kept = append(kept, kv)
		}
	}
	return kept
}

func (s redactedSpan) Status() sdktrace.Status {
	return sdktrace.Status{Code: s.ReadOnlySpan.Status().Code}
}

func (s redactedSpan) Events() []sdktrace.Event {
	original := s.ReadOnlySpan.Events()
	events := make([]sdktrace.Event, len(original))
	for i, event := range original {
		attributes := make([]attribute.KeyValue, 0, len(event.Attributes))
		for _, kv := range event.Attributes {
			if !exceptionDetailKeys[kv.Key] {
				attributes = append(attributes, kv)
			}
		}
		event.Attributes = attributes
		events[i] = event
	}
	return events
}

func orRedacted(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// withoutDetail keeps the origin of a full URL and trades its path, query and
// fragment for the template: otelhttp.NewTransport records the whole URL
// (otelhttp@v0.72.0/internal/semconv/client.go:114-123).
func withoutDetail(full, template string) string {
	parsed, err := url.Parse(full)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return redacted
	}
	return parsed.Scheme + "://" + parsed.Host + orRedacted(template, "/"+redacted)
}
