package app_test

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var semconvDurationBoundaries = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func meteredConsumer(handler *fakeHandler) (app.Consumer, *sdkmetric.ManualReader) {
	reader := sdkmetric.NewManualReader()
	consumer, _, _ := tracedConsumer(handler.handle, &fakeContainment{}, sdktrace.AlwaysSample())
	consumer.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	return consumer, reader
}

func processDurations(t *testing.T, reader *sdkmetric.ManualReader) []metricdata.HistogramDataPoint[float64] {
	t.Helper()

	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "messaging.process.duration" {
				continue
			}
			if m.Unit != "s" {
				t.Fatalf("unit = %q, want s", m.Unit)
			}
			histogram, ok := m.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("data = %T, want a float64 histogram", m.Data)
			}
			return histogram.DataPoints
		}
	}
	return nil
}

func onlyDuration(t *testing.T, reader *sdkmetric.ManualReader) metricdata.HistogramDataPoint[float64] {
	t.Helper()

	points := processDurations(t, reader)
	if len(points) != 1 || points[0].Count != 1 {
		t.Fatalf("messaging.process.duration = %+v, want one process timed", points)
	}
	return points[0]
}

func TestTheProcessIsTimedWithTheMessagingAttributes(t *testing.T) {
	transient := application.NewFailure(application.TransientDependency, true, errHandler)
	terminal := application.NewFailure(application.Unexpected, false, errHandler)
	rejection := application.NewFailure(application.DomainRejection, false, errHandler)
	invalid := application.NewFailure(application.Validation, false, errHandler)

	for _, tc := range []struct {
		name        string
		disposition application.Disposition
		handleErr   error
		ackErr      error
		errorType   string
	}{
		{name: "acknowledged", disposition: application.R1D1},
		{name: "released for another attempt", disposition: application.R1D3, handleErr: transient},
		{name: "contained as terminal", disposition: application.R1D4, handleErr: terminal, errorType: string(application.Unexpected)},
		{name: "a domain rejection contained as terminal", disposition: application.R1D4, handleErr: rejection, errorType: string(application.DomainRejection)},
		{name: "a validation contained as terminal", disposition: application.R1D4, handleErr: invalid, errorType: string(application.Validation)},
		{name: "acknowledgement failed", disposition: application.R1D1, ackErr: errAck, errorType: semconv.ErrorTypeOther.Value.AsString()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := validRaw(t)
			consumer, reader := meteredConsumer(&fakeHandler{disposition: tc.disposition, err: tc.handleErr})

			_, _ = consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{err: tc.ackErr})
			point := onlyDuration(t, reader)

			want := map[string]string{
				"messaging.operation.name":      "process",
				"messaging.system":              "kafka",
				"messaging.destination.name":    testTopic,
				"messaging.consumer.group.name": testGroup,
			}
			if tc.errorType != "" {
				want["error.type"] = tc.errorType
			}
			if point.Attributes.Len() != len(want) {
				t.Errorf("attributes = %v, want exactly %v", point.Attributes.ToSlice(), want)
			}
			for key, value := range want {
				if got, ok := point.Attributes.Value(attribute.Key(key)); !ok || got.AsString() != value {
					t.Errorf("%s = %q (present %v), want %q", key, got.AsString(), ok, value)
				}
			}
			if !slices.Equal(point.Bounds, semconvDurationBoundaries) {
				t.Fatalf("bounds = %v, want the semconv advisory %v (RF-D4)", point.Bounds, semconvDurationBoundaries)
			}
		})
	}
}

func TestAProcessRefusedAtTheBoundaryIsTimedToo(t *testing.T) {
	raw, _ := validRaw(t)
	consumer, reader := meteredConsumer(&fakeHandler{disposition: application.R1D1})
	consumer.Boundary.Sources = []string{"urn:dmpf:someone-else"}

	if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}
	point := onlyDuration(t, reader)

	if got, _ := point.Attributes.Value("messaging.destination.name"); got.AsString() != testTopic {
		t.Fatalf("messaging.destination.name = %q, want %q", got.AsString(), testTopic)
	}
}

func TestAnInvalidEnvelopeIsNotTimedAsAProcess(t *testing.T) {
	consumer, reader := meteredConsumer(&fakeHandler{disposition: application.R1D1})

	_, _ = consumer.Consume(context.Background(), app.Delivery{Raw: []byte("not an envelope"), Attempt: 1}, &fakeAck{})

	if points := processDurations(t, reader); len(points) != 0 {
		t.Fatalf("messaging.process.duration = %+v, want nothing without a process", points)
	}
}

func TestAConsumerWithoutAMeterProviderReportsToTheGlobalOne(t *testing.T) {
	previous := otel.GetMeterProvider()
	t.Cleanup(func() { otel.SetMeterProvider(previous) })
	reader := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	raw, _ := validRaw(t)
	consumer, _, _ := tracedConsumer((&fakeHandler{disposition: application.R1D1}).handle, &fakeContainment{}, sdktrace.AlwaysSample())

	if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}

	onlyDuration(t, reader)
}

func TestTheProcessDurationCoversTheHandling(t *testing.T) {
	const hold = 20 * time.Millisecond
	slow := func(context.Context, ports.Receipt, envelope.Envelope) (application.Disposition, error) {
		time.Sleep(hold)
		return application.R1D1, nil
	}
	reader := sdkmetric.NewManualReader()
	consumer, _, _ := tracedConsumer(slow, &fakeContainment{}, sdktrace.AlwaysSample())
	consumer.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	raw, _ := validRaw(t)

	if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("Consume() = %v", err)
	}

	if point := onlyDuration(t, reader); point.Sum < hold.Seconds() {
		t.Fatalf("messaging.process.duration sum = %v, want at least %v", point.Sum, hold)
	}
}

type countingMeterProvider struct {
	metric.MeterProvider
	meters atomic.Int64
}

func (p *countingMeterProvider) Meter(name string, options ...metric.MeterOption) metric.Meter {
	p.meters.Add(1)
	return p.MeterProvider.Meter(name, options...)
}

func TestTheProcessInstrumentIsBuiltOncePerProviderNotPerMessage(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := &countingMeterProvider{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	consumer, _, _ := tracedConsumer((&fakeHandler{disposition: application.R1D1}).handle, &fakeContainment{}, sdktrace.AlwaysSample())
	consumer.MeterProvider = provider

	for range 3 {
		raw, _ := validRaw(t)
		if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
			t.Fatalf("Consume() = %v", err)
		}
	}

	if got := provider.meters.Load(); got != 1 {
		t.Fatalf("Meter() called %d times for 3 messages, want once for the provider", got)
	}
	if points := processDurations(t, reader); len(points) != 1 || points[0].Count != 3 {
		t.Fatalf("messaging.process.duration = %+v, want the 3 processes timed on the instrument built once", points)
	}
}
