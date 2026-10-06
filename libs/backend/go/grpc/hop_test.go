package grpc_test

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
)

func TestValidCorrelationBoundsWhatACallerMayNameTheChain(t *testing.T) {
	for _, valid := range []string{"c-1", "A.b_c-9", strings.Repeat("a", 128)} {
		if !kernelgrpc.ValidCorrelation(valid) {
			t.Errorf("ValidCorrelation(%q) = false, want true", valid)
		}
	}
	for _, invalid := range []string{"", "has space", "slash/inside", strings.Repeat("a", 129), "line\nbreak"} {
		if kernelgrpc.ValidCorrelation(invalid) {
			t.Errorf("ValidCorrelation(%q) = true, want false", invalid)
		}
	}
}

func TestValidLocaleAcceptsOnlyALanguageTagWithoutParameters(t *testing.T) {
	for _, valid := range []string{"en", "pt-BR", "zh-Hant-TW"} {
		if !kernelgrpc.ValidLocale(valid) {
			t.Errorf("ValidLocale(%q) = false, want true", valid)
		}
	}
	for _, invalid := range []string{"", "pt_BR", "en;q=0.9", "toolonglang", "en-"} {
		if kernelgrpc.ValidLocale(invalid) {
			t.Errorf("ValidLocale(%q) = true, want false", invalid)
		}
	}
}

func TestNewIDIsSixteenRandomBytesInHex(t *testing.T) {
	hex := regexp.MustCompile(`^[0-9a-f]{32}$`)
	first, second := kernelgrpc.NewID("grpc"), kernelgrpc.NewID("grpc")

	if !hex.MatchString(first) || !hex.MatchString(second) {
		t.Fatalf("NewID() = %q, %q, want 32 lowercase hex characters", first, second)
	}
	if first == second {
		t.Fatal("NewID() repeated an identifier")
	}
	if !kernelgrpc.ValidCorrelation(first) {
		t.Fatal("a generated identifier must be a valid correlation")
	}
}

func TestMetadataCarrierCarriesTheTraceContextAcrossTheHop(t *testing.T) {
	span := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})
	md := metadata.MD{}

	propagation.TraceContext{}.Inject(trace.ContextWithSpanContext(context.Background(), span), kernelgrpc.MetadataCarrier(md))

	if got := md.Get("traceparent"); len(got) != 1 {
		t.Fatalf("traceparent = %v, want one value written through the carrier", got)
	}
	carried := trace.SpanContextFromContext(propagation.TraceContext{}.Extract(context.Background(), kernelgrpc.MetadataCarrier(md)))
	if carried.TraceID() != span.TraceID() || carried.SpanID() != span.SpanID() {
		t.Fatalf("extracted %v, want %v", carried, span)
	}
	if keys := kernelgrpc.MetadataCarrier(md).Keys(); !slices.Equal(keys, []string{"traceparent"}) {
		t.Fatalf("Keys() = %v, want [traceparent]", keys)
	}
	if got := kernelgrpc.MetadataCarrier(md).Get("absent"); got != "" {
		t.Fatalf("Get(absent) = %q, want empty", got)
	}
}
