package otelboot_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

func exportThroughPrivacy(t *testing.T, stubs ...tracetest.SpanStub) tracetest.SpanStubs {
	t.Helper()
	inner := tracetest.NewInMemoryExporter()
	spans := make([]sdktrace.ReadOnlySpan, 0, len(stubs))
	for _, stub := range stubs {
		spans = append(spans, stub.Snapshot())
	}
	if err := otelboot.NewPrivacyExporter(inner).ExportSpans(context.Background(), spans); err != nil {
		t.Fatalf("ExportSpans() = %v", err)
	}
	return inner.GetSpans()
}

func attributesOf(span tracetest.SpanStub) map[attribute.Key]attribute.Value {
	indexed := make(map[attribute.Key]attribute.Value, len(span.Attributes))
	for _, kv := range span.Attributes {
		indexed[kv.Key] = kv.Value
	}
	return indexed
}

func TestTheServerSpanLeavesWithoutThePeerAndWithTheRouteAsPath(t *testing.T) {
	exported := exportThroughPrivacy(t, tracetest.SpanStub{
		Name:     "POST /orders/{id}/place",
		SpanKind: trace.SpanKindServer,
		Attributes: []attribute.KeyValue{
			semconv.ClientAddress("203.0.113.9"),
			semconv.NetworkPeerAddress("10.0.0.7"),
			semconv.NetworkPeerPort(51234),
			semconv.UserAgentOriginal("curl/8"),
			semconv.HTTPRoute("/orders/{id}/place"),
			semconv.URLPath("/orders/123/place"),
		},
	})

	attributes := attributesOf(exported[0])
	for _, key := range []attribute.Key{semconv.ClientAddressKey, semconv.NetworkPeerAddressKey,
		semconv.NetworkPeerPortKey, semconv.UserAgentOriginalKey} {
		if _, present := attributes[key]; present {
			t.Errorf("%s left the process", key)
		}
	}
	if got := attributes[semconv.URLPathKey].AsString(); got != "/orders/{id}/place" {
		t.Errorf("url.path = %q, want the template of http.route", got)
	}
}

func TestAPathWithoutARouteIsRedacted(t *testing.T) {
	exported := exportThroughPrivacy(t, tracetest.SpanStub{
		Name:       "GET",
		Attributes: []attribute.KeyValue{semconv.URLPath("/orders/123")},
	})

	if got := attributesOf(exported[0])[semconv.URLPathKey].AsString(); got != "REDACTED" {
		t.Errorf("url.path = %q, want REDACTED", got)
	}
}

func TestTheFullURLOfAClientSpanKeepsOnlyTheOriginAndTheTemplate(t *testing.T) {
	exported := exportThroughPrivacy(t,
		tracetest.SpanStub{
			Name:       "GET",
			SpanKind:   trace.SpanKindClient,
			Attributes: []attribute.KeyValue{semconv.URLFull("https://idp/realms/x/certs?token=s#frag")},
		},
		tracetest.SpanStub{
			Name:     "GET",
			SpanKind: trace.SpanKindClient,
			Attributes: []attribute.KeyValue{
				semconv.URLFull("https://idp/realms/x/certs?token=s"),
				semconv.URLTemplate("/realms/{realm}/certs"),
			},
		},
	)

	if got := attributesOf(exported[0])[semconv.URLFullKey].AsString(); got != "https://idp/REDACTED" {
		t.Errorf("url.full without url.template = %q, want https://idp/REDACTED", got)
	}
	if got := attributesOf(exported[1])[semconv.URLFullKey].AsString(); got != "https://idp/realms/{realm}/certs" {
		t.Errorf("url.full with url.template = %q, want the origin and the template", got)
	}
}

func TestTheStatusDescriptionAndTheExceptionDetailNeverLeave(t *testing.T) {
	exported := exportThroughPrivacy(t, tracetest.SpanStub{
		Name:   "orders.place",
		Status: sdktrace.Status{Code: codes.Error, Description: "pq: duplicate key value 42"},
		Events: []sdktrace.Event{{
			Name: semconv.ExceptionEventName,
			Attributes: []attribute.KeyValue{
				semconv.ExceptionType("*pgconn.PgError"),
				semconv.ExceptionMessage("duplicate key value 42"),
				semconv.ExceptionStacktrace("goroutine 1"),
			},
		}},
	})

	span := exported[0]
	if span.Status.Code != codes.Error || span.Status.Description != "" {
		t.Errorf("Status = %+v, want the code kept and the description empty", span.Status)
	}
	event := map[attribute.Key]bool{}
	for _, kv := range span.Events[0].Attributes {
		event[kv.Key] = true
	}
	if event[semconv.ExceptionMessageKey] || event[semconv.ExceptionStacktraceKey] || !event[semconv.ExceptionTypeKey] {
		t.Errorf("exception event attributes = %v, want only exception.type", span.Events[0].Attributes)
	}
}

func TestACleanSpanPassesIdenticalWithItsLinks(t *testing.T) {
	link := trace.Link{SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{7}, SpanID: trace.SpanID{7}, TraceFlags: trace.FlagsSampled,
	})}
	clean := tracetest.SpanStub{
		Name:       "orders.place",
		Attributes: []attribute.KeyValue{semconv.HTTPRoute("/orders"), attribute.String("dmpf.correlation_id", "c-1")},
		Links:      []sdktrace.Link{{SpanContext: link.SpanContext}},
		Status:     sdktrace.Status{Code: codes.Ok},
	}

	exported := exportThroughPrivacy(t, clean)[0]

	if len(exported.Attributes) != 2 || exported.Attributes[0] != clean.Attributes[0] || exported.Attributes[1] != clean.Attributes[1] {
		t.Errorf("Attributes = %v, want %v untouched", exported.Attributes, clean.Attributes)
	}
	if len(exported.Links) != 1 || exported.Links[0].SpanContext.SpanID() != link.SpanContext.SpanID() {
		t.Errorf("Links = %v, want the link preserved", exported.Links)
	}
}

type flushingExporter struct {
	sdktrace.SpanExporter
	flushed, closed bool
}

func (e *flushingExporter) ForceFlush(context.Context) error { e.flushed = true; return nil }
func (e *flushingExporter) Shutdown(context.Context) error {
	e.closed = true
	return errors.New("closed")
}

func TestShutdownAndForceFlushReachTheInnerExporter(t *testing.T) {
	inner := &flushingExporter{SpanExporter: tracetest.NewInMemoryExporter()}
	decorated := otelboot.NewPrivacyExporter(inner)

	flusher, ok := decorated.(interface{ ForceFlush(context.Context) error })
	if !ok {
		t.Fatal("the decorator hides ForceFlush")
	}
	if err := flusher.ForceFlush(context.Background()); err != nil || !inner.flushed {
		t.Errorf("ForceFlush() = %v, flushed = %v; want it delegated", err, inner.flushed)
	}
	if err := decorated.Shutdown(context.Background()); err == nil || !inner.closed {
		t.Errorf("Shutdown() = %v, closed = %v; want it delegated with its result", err, inner.closed)
	}
}

func TestTheRuntimeExportsThroughThePrivacyDecorator(t *testing.T) {
	runtime, exporter := startedRuntime(t, nil)

	_, span := runtime.Tracer().Start(context.Background(), "orders.place",
		trace.WithAttributes(semconv.ClientAddress("203.0.113.9")))
	span.SetStatus(codes.Error, "secret detail")
	span.End()
	if err := runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	got := exporter.GetSpans()
	if len(got) != 1 {
		t.Fatalf("exported %d spans, want 1", len(got))
	}
	if _, present := attributesOf(got[0])[semconv.ClientAddressKey]; present || got[0].Status.Description != "" {
		t.Errorf("exported = %+v, want it redacted before the exporter", got[0])
	}
}
