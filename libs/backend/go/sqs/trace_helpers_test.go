package sqs_test

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
)

func traced(cfg provider.Config) (provider.Config, *tracetest.SpanRecorder, trace.Tracer) {
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("sqs-test")
	cfg.Tracer = tracer
	return cfg, recorder, tracer
}

func underOwnedSend(tracer trace.Tracer) (context.Context, trace.Span) {
	ctx, span := tracer.Start(context.Background(), "send relay-owned", trace.WithSpanKind(trace.SpanKindClient))
	return tracing.WithOwnedSpan(ctx, span), span
}

func attributeOf(span sdktrace.ReadOnlySpan, key attribute.Key) (attribute.Value, bool) {
	for _, kv := range span.Attributes() {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func assertNoProviderSpanName(t *testing.T, spans []sdktrace.ReadOnlySpan) {
	t.Helper()
	for _, span := range spans {
		if strings.HasPrefix(span.Name(), "dmpf.sqs.") || strings.HasPrefix(span.Name(), "dmpf.sns.") {
			t.Errorf("span %q: the provider names no span of its own (RF-B7)", span.Name())
		}
	}
}

func assertDestinationOnTheResilienceSpan(t *testing.T, recorder *tracetest.SpanRecorder, system attribute.KeyValue, destination string) {
	t.Helper()
	spans := recorder.Ended()
	assertNoProviderSpanName(t, spans)
	if len(spans) != 1 || spans[0].Name() != "dmpf.resilience sqs" {
		names := make([]string, len(spans))
		for i, span := range spans {
			names[i] = span.Name()
		}
		t.Fatalf("ended spans = %q, want only the resilience span over the attempts (RF-B5)", names)
	}
	if got, _ := attributeOf(spans[0], semconv.MessagingSystemKey); got != system.Value {
		t.Errorf("messaging.system = %q, want %q outside the relay (RF-B7)", got.String(), system.Value.String())
	}
	if got, _ := attributeOf(spans[0], semconv.MessagingDestinationNameKey); got.AsString() != destination {
		t.Errorf("messaging.destination.name = %q, want the physical %q (RF-B7)", got.AsString(), destination)
	}
	for _, kv := range spans[0].Attributes() {
		if strings.Contains(kv.Value.String(), awsAccount) {
			t.Errorf("%s = %q carries the AWS account of the address; the destination is named by the queue or the topic (RF-B7)", kv.Key, kv.Value.String())
		}
	}
}

const awsAccount = "000000000000"

func assertTheOwnedSendIsReused(t *testing.T, recorder *tracetest.SpanRecorder) {
	t.Helper()
	spans := recorder.Ended()
	assertNoProviderSpanName(t, spans)
	if len(spans) != 1 || spans[0].Name() != "send relay-owned" {
		t.Fatalf("ended %d spans, want only the relay's send: the provider reuses the span it owns (RF-B5)", len(spans))
	}
	if _, ok := attributeOf(spans[0], tracing.KeyDependency); !ok {
		t.Error("the resilience attributes did not land on the owned send")
	}
	for _, key := range []attribute.Key{semconv.MessagingSystemKey, semconv.MessagingDestinationNameKey} {
		if got, ok := attributeOf(spans[0], key); ok {
			t.Errorf("%s = %q on the relay's send: the owner writes messaging.*, never the provider", key, got.String())
		}
	}
}
