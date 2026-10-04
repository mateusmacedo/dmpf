package app

import (
	"context"
	"reflect"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/messagingconv"
)

const meterName = "github.com/mateusmacedo/dmpf/libs/backend/go/app"

var durationBoundaries = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func (c Consumer) meterProvider() metric.MeterProvider {
	if c.MeterProvider == nil {
		return otel.GetMeterProvider()
	}
	return c.MeterProvider
}

var processInstruments sync.Map

func (c Consumer) processDuration() messagingconv.ProcessDuration {
	provider := c.meterProvider()
	if !reflect.ValueOf(provider).Comparable() {
		return newProcessDuration(provider)
	}
	if cached, ok := processInstruments.Load(provider); ok {
		return cached.(messagingconv.ProcessDuration)
	}
	cached, _ := processInstruments.LoadOrStore(provider, newProcessDuration(provider))
	return cached.(messagingconv.ProcessDuration)
}

func newProcessDuration(provider metric.MeterProvider) messagingconv.ProcessDuration {
	duration, _ := messagingconv.NewProcessDuration(provider.Meter(meterName),
		metric.WithExplicitBucketBoundaries(durationBoundaries...))
	return duration
}

func (c Consumer) measureProcess(ctx context.Context, began time.Time, outcome Outcome, gesture *gestures, err error) {
	var attributes []attribute.KeyValue
	if c.Channel.Address != "" {
		attributes = append(attributes, semconv.MessagingDestinationName(c.Channel.Address))
	}
	if c.Channel.Group != "" {
		attributes = append(attributes, semconv.MessagingConsumerGroupName(c.Channel.Group))
	}
	if failed(outcome, gesture, err) {
		attributes = append(attributes, semconv.ErrorTypeKey.String(outcomeCategory(err)))
	}

	c.processDuration().Record(ctx, time.Since(began).Seconds(), processOperation, messagingconv.SystemAttr(c.System), attributes...)
}
