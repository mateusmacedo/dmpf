package relay

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/messagingconv"
)

const meterName = "github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"

var durationBoundaries = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func (r Relay) meterProvider() metric.MeterProvider {
	if r.MeterProvider == nil {
		return otel.GetMeterProvider()
	}
	return r.MeterProvider
}

type sendInstruments struct {
	duration messagingconv.ClientOperationDuration
	sent     messagingconv.ClientSentMessages
}

func (r Relay) instruments() *sendInstruments {
	if r.meters != nil {
		return r.meters
	}
	meter := r.meterProvider().Meter(meterName)
	duration, _ := messagingconv.NewClientOperationDuration(meter, metric.WithExplicitBucketBoundaries(durationBoundaries...))
	sent, _ := messagingconv.NewClientSentMessages(meter)
	return &sendInstruments{duration: duration, sent: sent}
}

// A record that never reached the broker is not counted as sent: semconv forbids
// counting messages created but not sent (messagingconv@v1.43.0/metric.go:658).
func (r Relay) measureSend(ctx context.Context, began time.Time, destination, failure string, reachedBroker bool) {
	instruments := r.instruments()
	system := messagingconv.SystemAttr(r.System)

	var attributes []attribute.KeyValue
	if address := r.address(destination); address != "" {
		attributes = append(attributes, semconv.MessagingDestinationName(address))
	}
	if failure != "" {
		attributes = append(attributes, semconv.ErrorTypeKey.String(failure))
	}

	instruments.duration.Record(ctx, time.Since(began).Seconds(), sendOperation, system,
		append(attributes, instruments.duration.AttrOperationType(messagingconv.OperationTypeSend))...)
	if reachedBroker {
		instruments.sent.Add(ctx, 1, sendOperation, system, attributes...)
	}
}
