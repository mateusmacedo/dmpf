package app

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
)

const anotherTypeDiscarded = "delivery of another event type discarded"

func (c Consumer) discard(ctx context.Context, env envelope.Envelope, gesture *gestures) (Outcome, error) {
	if err := gesture.Ack(ctx); err != nil {
		return Outcome{}, err
	}
	labels := metrics.Labels{}.MessagingSystem(c.System).Destination(c.Channel.Address).ConsumerGroup(c.Channel.Group).EventType(env.Type)
	c.discarded().Add(ctx, 1, metric.WithAttributes(labels.Attributes()...))
	c.logger().LogAttrs(ctx, slog.LevelDebug, anotherTypeDiscarded,
		slog.String(string(semconv.CloudEventsEventTypeKey), env.Type),
		slog.String(string(semconv.MessagingMessageIDKey), env.ID))
	return Outcome{Discarded: true}, nil
}
