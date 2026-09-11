package otelboot

import (
	"encoding/binary"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/tracing"
)

// classSampler decides by traffic class and never drops a span (TRC-13,
// TRC-14). It is written from scratch rather than composed from ParentBased and
// TraceIDRatioBased because the negative branch of both returns Drop, and a
// dropped span is never created — so no processor could retain it when it later
// fails.
type classSampler struct {
	upperBounds map[tracing.Class]uint64
	rates       tracing.Rates
	description string
}

// NewClassSampler returns the platform sampler for the given rates. A class the
// table does not name takes the most restrictive rate, and a span that declares
// no class is treated as unclassified and says so on the span itself.
func NewClassSampler(rates tracing.Rates) sdktrace.Sampler {
	own := maps.Clone(rates)
	if own == nil {
		own = tracing.Rates{}
	}

	upperBounds := make(map[tracing.Class]uint64, len(own)+1)
	for class, rate := range own {
		upperBounds[class] = upperBound(rate)
	}
	upperBounds[tracing.ClassUnclassified] = upperBound(own.RateFor(tracing.ClassUnclassified))

	return classSampler{upperBounds: upperBounds, rates: own, description: describeRates(own)}
}

// upperBound is the threshold of TraceIDRatioBased, kept bit for bit
// (sdk/trace/sampling.go:114) so a trace sampled here would be sampled the same
// way by any other service running the SDK sampler at the same rate.
//
// A rate that is not a number is refused before the arithmetic: NaN fails every
// comparison, so it would fall through to a bound of 2^63 — above any value the
// trace ID can produce — and quietly sample everything. It resolves to the most
// restrictive rate instead, which is what an undeclared class already does.
func upperBound(rate float64) uint64 {
	if math.IsNaN(rate) {
		rate = tracing.RateMostRestrictive
	}
	if rate >= 1 {
		return ^uint64(0)
	}
	if rate <= 0 {
		return 0
	}
	return uint64(rate * (1 << 63))
}

func (s classSampler) ShouldSample(parameters sdktrace.SamplingParameters) sdktrace.SamplingResult {
	parent := trace.SpanContextFromContext(parameters.ParentContext)
	result := sdktrace.SamplingResult{Tracestate: parent.TraceState()}

	if parent.IsValid() {
		// A child follows its parent, local or remote: a trace that is being
		// sampled stays whole, and one that is not still records, so an error
		// deeper in the call keeps its own span (TRC-14).
		result.Decision = decide(parent.IsSampled())
		return result
	}

	class, declared := trafficClass(parameters.Attributes)
	if !declared {
		result.Attributes = []attribute.KeyValue{
			attribute.String(tracing.KeyTrafficClass, string(tracing.ClassUnclassified)),
		}
	}

	result.Decision = decide(s.within(class, parameters.TraceID))
	return result
}

// within is the ratio test of the SDK: the same eight bytes, the same shift and
// the same comparison.
func (s classSampler) within(class tracing.Class, traceID trace.TraceID) bool {
	bound, known := s.upperBounds[class]
	if !known {
		bound = upperBound(tracing.RateMostRestrictive)
	}
	return binary.BigEndian.Uint64(traceID[8:16])>>1 < bound
}

// decide turns the rate test into a decision. The negative branch is RecordOnly
// and never Drop: the span exists, unsampled, and the processor exports it if it
// ends in error.
func decide(sampled bool) sdktrace.SamplingDecision {
	if sampled {
		return sdktrace.RecordAndSample
	}
	return sdktrace.RecordOnly
}

func trafficClass(attributes []attribute.KeyValue) (tracing.Class, bool) {
	for _, attr := range attributes {
		if string(attr.Key) == tracing.KeyTrafficClass && attr.Value.AsString() != "" {
			return tracing.Class(attr.Value.AsString()), true
		}
	}
	return tracing.ClassUnclassified, false
}

func (s classSampler) Description() string { return s.description }

func describeRates(rates tracing.Rates) string {
	stated := make([]string, 0, len(rates))
	for _, class := range slices.Sorted(maps.Keys(rates)) {
		stated = append(stated, fmt.Sprintf("%s=%g", class, rates[class]))
	}
	return fmt.Sprintf("DMPFClassSampler{%s;absent=%g}", strings.Join(stated, ","), tracing.RateMostRestrictive)
}
