package otelboot_test

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
)

const (
	payloadKey = attribute.Key("dmpf.test.payload")
	idKey      = attribute.Key("dmpf.test.id")
	shortID    = "0b7e3a52-5c3f-4f8e-9d2a-1c6b8e4f7a10"
)

var oversized = strings.Repeat("x", 4096)

func exportedLogAttributes(t *testing.T) map[attribute.Key]string {
	t.Helper()
	exporter := &recordingExporter{}
	provider := otelboot.NewLoggerProvider(validConfig(), exporter)

	var record log.Record
	record.SetBody(attribute.StringValue("placed"))
	record.AddAttributes(attribute.String(string(payloadKey), oversized), attribute.String(string(idKey), shortID))
	provider.Logger("test").Emit(context.Background(), record)
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}

	if len(exporter.records) != 1 {
		t.Fatalf("exported %d records, want 1", len(exporter.records))
	}
	attributes := map[attribute.Key]string{}
	exporter.records[0].WalkAttributes(func(kv attribute.KeyValue) bool {
		attributes[kv.Key] = kv.Value.AsString()
		return true
	})
	return attributes
}

func exportedSpanAttributes(t *testing.T) map[attribute.Key]string {
	t.Helper()
	runtime, exporter := startedRuntime(t, nil)

	_, span := runtime.Tracer().Start(context.Background(), "orders.place",
		trace.WithAttributes(attribute.String(string(payloadKey), oversized), attribute.String(string(idKey), shortID)))
	span.End()
	if err := runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want 1", len(spans))
	}
	attributes := map[attribute.Key]string{}
	for _, kv := range spans[0].Attributes {
		attributes[kv.Key] = kv.Value.AsString()
	}
	return attributes
}

func TestAnAttributeValueIsCutAtTheLengthLimitOfEachSignal(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		wantLog  int
		wantSpan int
	}{
		{name: "no environment", wantLog: 1024, wantSpan: 1024},
		{
			name:    "general limit",
			env:     map[string]string{"OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT": "512"},
			wantLog: 512, wantSpan: 512,
		},
		{
			name:    "span limit",
			env:     map[string]string{"OTEL_SPAN_ATTRIBUTE_VALUE_LENGTH_LIMIT": "2048"},
			wantLog: 1024, wantSpan: 2048,
		},
		{
			name: "span limit over the general one",
			env: map[string]string{
				"OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT":      "512",
				"OTEL_SPAN_ATTRIBUTE_VALUE_LENGTH_LIMIT": "2048",
			},
			wantLog: 512, wantSpan: 2048,
		},
		{
			name:    "log record limit",
			env:     map[string]string{"OTEL_LOGRECORD_ATTRIBUTE_VALUE_LENGTH_LIMIT": "2048"},
			wantLog: 2048, wantSpan: 1024,
		},
		{
			name: "log record limit over the general one",
			env: map[string]string{
				"OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT":           "512",
				"OTEL_LOGRECORD_ATTRIBUTE_VALUE_LENGTH_LIMIT": "2048",
			},
			wantLog: 2048, wantSpan: 512,
		},
		{
			name:    "unlimited by the environment",
			env:     map[string]string{"OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT": "-1"},
			wantLog: 4096, wantSpan: 4096,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{
				"OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT",
				"OTEL_SPAN_ATTRIBUTE_VALUE_LENGTH_LIMIT",
				"OTEL_LOGRECORD_ATTRIBUTE_VALUE_LENGTH_LIMIT",
			} {
				t.Setenv(key, tc.env[key])
			}

			signals := map[string]struct {
				attributes map[attribute.Key]string
				want       int
			}{
				"log record": {exportedLogAttributes(t), tc.wantLog},
				"span":       {exportedSpanAttributes(t), tc.wantSpan},
			}
			for signal, got := range signals {
				if length := len(got.attributes[payloadKey]); length != got.want {
					t.Errorf("%s: %s has %d bytes, want %d", signal, payloadKey, length, got.want)
				}
				if id := got.attributes[idKey]; id != shortID {
					t.Errorf("%s: %s = %q, want %q intact", signal, idKey, id, shortID)
				}
			}
		})
	}
}
