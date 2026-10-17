package authn

import (
	"context"
	"reflect"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const keyRevocationKind = "kind"

type revocationMetrics struct {
	refusals    metric.Int64Counter
	storeErrors metric.Int64Counter
}

func newRevocationMetrics(provider metric.MeterProvider) revocationMetrics {
	if provider == nil {
		provider = otel.GetMeterProvider()
	}
	meter := provider.Meter(reflect.TypeFor[Verifier]().PkgPath())
	refusals, _ := meter.Int64Counter("dmpf.auth.revocation.refusals",
		metric.WithUnit("{refusal}"), metric.WithDescription("Tokens refused because the deny-list revoked them."))
	storeErrors, _ := meter.Int64Counter("dmpf.auth.revocation.store_errors",
		metric.WithUnit("{error}"), metric.WithDescription("Deny-list lookups that failed or timed out."))
	return revocationMetrics{refusals: refusals, storeErrors: storeErrors}
}

func (m revocationMetrics) refused(ctx context.Context, kind RevocationKind) {
	if kind != RevocationSession {
		kind = RevocationSubject
	}
	m.refusals.Add(ctx, 1, metric.WithAttributes(attribute.String(keyRevocationKind, string(kind))))
}

func (m revocationMetrics) storeFailed(ctx context.Context) {
	m.storeErrors.Add(ctx, 1)
}

const keyEventOutcome = "outcome"

var applyDelayBoundaries = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 30, 45, 60, 120, 300}

type feedMetrics struct {
	events     metric.Int64Counter
	applyDelay metric.Float64Histogram
}

func newFeedMetrics(provider metric.MeterProvider) feedMetrics {
	if provider == nil {
		provider = otel.GetMeterProvider()
	}
	meter := provider.Meter(reflect.TypeFor[Feed]().PkgPath())
	events, _ := meter.Int64Counter("dmpf.auth.revocation.events",
		metric.WithUnit("{event}"), metric.WithDescription("Session events received by the revocation feed, by outcome."))
	applyDelay, _ := meter.Float64Histogram("dmpf.auth.revocation.apply_delay",
		metric.WithUnit("s"), metric.WithDescription("Time from the session event to its deny-list entry."),
		metric.WithExplicitBucketBoundaries(applyDelayBoundaries...))
	return feedMetrics{events: events, applyDelay: applyDelay}
}

func (m feedMetrics) observed(ctx context.Context, outcome EventOutcome) {
	m.events.Add(ctx, 1, metric.WithAttributes(attribute.String(keyEventOutcome, string(outcome))))
}

func (m feedMetrics) delayed(ctx context.Context, delay time.Duration) {
	m.applyDelay.Record(ctx, max(delay, 0).Seconds())
}
