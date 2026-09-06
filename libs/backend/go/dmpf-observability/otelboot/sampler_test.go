package otelboot_test

import (
	"context"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/otelboot"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
)

// traceIDWithin builds a TraceID the ratio formula of the SDK keeps: the eight
// bytes it reads are zero, so x is zero and falls under any positive rate.
func traceIDWithin() trace.TraceID {
	var id trace.TraceID
	id[0] = 1
	return id
}

// traceIDOutside builds a TraceID the ratio formula rejects for any rate below
// one: the eight bytes it reads are all set, so x is the largest it can be.
func traceIDOutside() trace.TraceID {
	var id trace.TraceID
	id[0] = 1
	binary.BigEndian.PutUint64(id[8:16], ^uint64(0))
	return id
}

func parentContext(t *testing.T, sampled, remote bool) context.Context {
	t.Helper()

	var flags trace.TraceFlags
	if sampled {
		flags = trace.FlagsSampled
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceIDOutside(),
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: flags,
		Remote:     remote,
	})
	if !spanContext.IsValid() {
		t.Fatal("the parent span context is not valid")
	}
	return trace.ContextWithSpanContext(context.Background(), spanContext)
}

func sample(sampler sdktrace.Sampler, id trace.TraceID, class tracing.Class) sdktrace.SamplingResult {
	parameters := sdktrace.SamplingParameters{
		ParentContext: context.Background(),
		TraceID:       id,
		Name:          "orders.place",
	}
	if class != "" {
		parameters.Attributes = []attribute.KeyValue{attribute.String(tracing.KeyTrafficClass, string(class))}
	}
	return sampler.ShouldSample(parameters)
}

func TestARootSpanWithinTheRateOfItsClassIsSampled(t *testing.T) {
	sampler := otelboot.NewClassSampler(tracing.DefaultRates())

	result := sample(sampler, traceIDWithin(), tracing.ClassWrite)
	if result.Decision != sdktrace.RecordAndSample {
		t.Errorf("Decision = %v, want RecordAndSample", result.Decision)
	}
}

func TestARootSpanOutsideTheRateOfItsClassIsRecordedButNotSampled(t *testing.T) {
	sampler := otelboot.NewClassSampler(tracing.DefaultRates())

	result := sample(sampler, traceIDOutside(), tracing.ClassWrite)
	if result.Decision != sdktrace.RecordOnly {
		t.Errorf("Decision = %v, want RecordOnly: the span is kept so an error can still be exported (TRC-14)", result.Decision)
	}
}

func TestTheSamplerNeverDropsASpan(t *testing.T) {
	sampler := otelboot.NewClassSampler(tracing.Rates{
		tracing.ClassError: 1.0,
		tracing.ClassWrite: 0,
		tracing.ClassRead:  0,
	})

	for _, testCase := range []struct {
		name string
		id   trace.TraceID
		of   tracing.Class
	}{
		{"rate zero, within", traceIDWithin(), tracing.ClassWrite},
		{"rate zero, outside", traceIDOutside(), tracing.ClassWrite},
		{"unclassified", traceIDOutside(), ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if decision := sample(sampler, testCase.id, testCase.of).Decision; decision == sdktrace.Drop {
				t.Errorf("Decision = Drop; the sampler must never drop (TRC-14)")
			}
		})
	}
}

func TestAClassRatedAtOneIsAlwaysSampled(t *testing.T) {
	sampler := otelboot.NewClassSampler(tracing.DefaultRates())

	for _, class := range []tracing.Class{tracing.ClassError, tracing.ClassMaintenance} {
		if decision := sample(sampler, traceIDOutside(), class).Decision; decision != sdktrace.RecordAndSample {
			t.Errorf("Decision for %q = %v, want RecordAndSample at rate 1.0", class, decision)
		}
	}
}

func TestASpanWithoutATrafficClassIsRatedAsTheMostRestrictiveAndSaysSo(t *testing.T) {
	sampler := otelboot.NewClassSampler(tracing.DefaultRates())

	result := sample(sampler, traceIDOutside(), "")
	if result.Decision != sdktrace.RecordOnly {
		t.Errorf("Decision = %v, want RecordOnly at the most restrictive rate", result.Decision)
	}

	if got := index(result.Attributes)[attribute.Key(tracing.KeyTrafficClass)]; got != string(tracing.ClassUnclassified) {
		t.Errorf("%s = %q, want %q: the omission has to be visible on the span",
			tracing.KeyTrafficClass, got, tracing.ClassUnclassified)
	}
}

func TestADeclaredClassIsNotRewrittenBySampling(t *testing.T) {
	sampler := otelboot.NewClassSampler(tracing.DefaultRates())

	result := sample(sampler, traceIDWithin(), tracing.ClassWrite)
	if _, rewritten := index(result.Attributes)[attribute.Key(tracing.KeyTrafficClass)]; rewritten {
		t.Errorf("the sampler added %s to a span that already declared its class", tracing.KeyTrafficClass)
	}
}

func TestTheParentDecidesForAChildSpan(t *testing.T) {
	sampler := otelboot.NewClassSampler(tracing.DefaultRates())

	for _, testCase := range []struct {
		name    string
		sampled bool
		remote  bool
		want    sdktrace.SamplingDecision
	}{
		{"local parent sampled", true, false, sdktrace.RecordAndSample},
		{"local parent not sampled", false, false, sdktrace.RecordOnly},
		{"remote parent sampled", true, true, sdktrace.RecordAndSample},
		{"remote parent not sampled", false, true, sdktrace.RecordOnly},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// The class would sample this trace ID at no rate below one, so a
			// RecordAndSample here can only have come from the parent.
			result := sampler.ShouldSample(sdktrace.SamplingParameters{
				ParentContext: parentContext(t, testCase.sampled, testCase.remote),
				TraceID:       traceIDOutside(),
				Name:          "orders.place",
				Attributes: []attribute.KeyValue{
					attribute.String(tracing.KeyTrafficClass, string(tracing.ClassRead)),
				},
			})
			if result.Decision != testCase.want {
				t.Errorf("Decision = %v, want %v", result.Decision, testCase.want)
			}
		})
	}
}

func TestTheSamplerCarriesTheParentTracestateThrough(t *testing.T) {
	state, err := trace.ParseTraceState("vendor=value")
	if err != nil {
		t.Fatalf("ParseTraceState() = %v", err)
	}
	parent := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceIDOutside(),
		SpanID:     trace.SpanID{1},
		TraceFlags: trace.FlagsSampled,
		TraceState: state,
	}))

	result := otelboot.NewClassSampler(tracing.DefaultRates()).ShouldSample(sdktrace.SamplingParameters{
		ParentContext: parent,
		TraceID:       traceIDOutside(),
		Name:          "orders.place",
	})
	if got := result.Tracestate.String(); got != "vendor=value" {
		t.Errorf("Tracestate = %q, want the parent's %q", got, "vendor=value")
	}
}

func TestTheSamplerDescribesItselfWithTheRatesInUse(t *testing.T) {
	description := otelboot.NewClassSampler(tracing.Rates{tracing.ClassWrite: 0.25}).Description()

	if !strings.Contains(description, "write=0.25") {
		t.Errorf("Description() = %q, want it to state the rate of each class", description)
	}
}

func TestTheRatesAreCopiedSoALaterWriteDoesNotChangeTheDecision(t *testing.T) {
	rates := tracing.Rates{tracing.ClassWrite: 1.0}
	sampler := otelboot.NewClassSampler(rates)

	rates[tracing.ClassWrite] = 0

	if decision := sample(sampler, traceIDOutside(), tracing.ClassWrite).Decision; decision != sdktrace.RecordAndSample {
		t.Errorf("Decision = %v, want RecordAndSample: the sampler reads the rates it was built with", decision)
	}
}

func TestAClassTheTableDoesNotNameTakesTheMostRestrictiveRate(t *testing.T) {
	sampler := otelboot.NewClassSampler(tracing.Rates{tracing.ClassError: 1.0})

	if decision := sample(sampler, traceIDOutside(), tracing.ClassWrite).Decision; decision != sdktrace.RecordOnly {
		t.Errorf("Decision = %v, want RecordOnly: an unnamed class must not pass everything through", decision)
	}
	if decision := sample(sampler, traceIDWithin(), tracing.ClassWrite).Decision; decision != sdktrace.RecordAndSample {
		t.Errorf("Decision = %v, want RecordAndSample: the restrictive rate is still a rate, not zero", decision)
	}
}

func TestASamplerBuiltWithoutRatesStillDecides(t *testing.T) {
	sampler := otelboot.NewClassSampler(nil)

	if decision := sample(sampler, traceIDOutside(), tracing.ClassError).Decision; decision != sdktrace.RecordOnly {
		t.Errorf("Decision = %v, want RecordOnly at the most restrictive rate", decision)
	}
}

// A rate that is not a number reaches the sampler from a configuration computed
// at boot — a ratio whose denominator turned out to be zero, say. It must not
// resolve to sampling everything: NaN fails every comparison, so a bound
// computed from it would let every trace through.
func TestARateThatIsNotANumberFallsBackToTheMostRestrictiveRate(t *testing.T) {
	sampler := otelboot.NewClassSampler(tracing.Rates{tracing.ClassWrite: math.NaN()})

	if decision := sample(sampler, traceIDOutside(), tracing.ClassWrite).Decision; decision != sdktrace.RecordOnly {
		t.Errorf("Decision = %v, want RecordOnly: NaN is not a rate, and must not mean 'sample everything'", decision)
	}
	if decision := sample(sampler, traceIDWithin(), tracing.ClassWrite).Decision; decision != sdktrace.RecordAndSample {
		t.Errorf("Decision = %v, want RecordAndSample: the fallback is the restrictive rate, not zero", decision)
	}
}
