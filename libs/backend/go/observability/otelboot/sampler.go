package otelboot

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

// NewClassSampler is AlwaysRecord(ParentBased(root)) (TRC-13, RF-E5): a child
// follows its parent, a root takes the rate of its traffic class, no span is
// dropped, and every local span without a class is marked unclassified.
func NewClassSampler(rates tracing.Rates) sdktrace.Sampler {
	own := maps.Clone(rates)
	if own == nil {
		own = tracing.Rates{}
	}

	byClass := make(map[tracing.Class]sdktrace.Sampler, len(own)+1)
	for class, rate := range own {
		byClass[class] = sdktrace.TraceIDRatioBased(sanitized(rate))
	}
	byClass[tracing.ClassUnclassified] = sdktrace.TraceIDRatioBased(sanitized(own.RateFor(tracing.ClassUnclassified)))

	root := classRoot{
		byClass:     byClass,
		absent:      sdktrace.TraceIDRatioBased(tracing.RateMostRestrictive),
		description: describeRates(own),
	}
	return classified{inner: sdktrace.AlwaysRecord(sdktrace.ParentBased(root))}
}

type classified struct {
	inner sdktrace.Sampler
}

func (c classified) ShouldSample(parameters sdktrace.SamplingParameters) sdktrace.SamplingResult {
	result := c.inner.ShouldSample(parameters)
	if _, declared := trafficClass(parameters.Attributes); !declared {
		result.Attributes = append(result.Attributes,
			attribute.String(tracing.KeyTrafficClass, string(tracing.ClassUnclassified)))
	}
	return result
}

func (c classified) Description() string { return c.inner.Description() }

// sanitized refuses a rate that is not a number: TraceIDRatioBased would turn
// NaN into a bound above any trace ID and sample everything.
func sanitized(rate float64) float64 {
	if math.IsNaN(rate) {
		return tracing.RateMostRestrictive
	}
	return rate
}

type classRoot struct {
	byClass     map[tracing.Class]sdktrace.Sampler
	absent      sdktrace.Sampler
	description string
}

func (r classRoot) ShouldSample(parameters sdktrace.SamplingParameters) sdktrace.SamplingResult {
	class, _ := trafficClass(parameters.Attributes)

	if followsLinks(class) && anySampled(parameters.Links) {
		return sdktrace.SamplingResult{
			Decision:   sdktrace.RecordAndSample,
			Tracestate: trace.SpanContextFromContext(parameters.ParentContext).TraceState(),
		}
	}
	return r.samplerOf(class).ShouldSample(parameters)
}

// followsLinks keeps the link rule off the refused boundary root, of class
// error: an external producer could otherwise force the sampling (RF-E5).
func followsLinks(class tracing.Class) bool {
	return class == tracing.ClassWrite || class == tracing.ClassRead
}

func anySampled(links []trace.Link) bool {
	return slices.ContainsFunc(links, func(link trace.Link) bool {
		return link.SpanContext.IsValid() && link.SpanContext.IsSampled()
	})
}

func (r classRoot) samplerOf(class tracing.Class) sdktrace.Sampler {
	if sampler, known := r.byClass[class]; known {
		return sampler
	}
	return r.absent
}

func trafficClass(attributes []attribute.KeyValue) (tracing.Class, bool) {
	for _, attr := range attributes {
		if string(attr.Key) == tracing.KeyTrafficClass && attr.Value.AsString() != "" {
			return tracing.Class(attr.Value.AsString()), true
		}
	}
	return tracing.ClassUnclassified, false
}

func (r classRoot) Description() string { return r.description }

func describeRates(rates tracing.Rates) string {
	stated := make([]string, 0, len(rates))
	for _, class := range slices.Sorted(maps.Keys(rates)) {
		stated = append(stated, fmt.Sprintf("%s=%g", class, rates[class]))
	}
	return fmt.Sprintf("DMPFClassSampler{%s;absent=%g}", strings.Join(stated, ","), tracing.RateMostRestrictive)
}
