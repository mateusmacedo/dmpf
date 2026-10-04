package otelboot

import (
	"slices"

	"go.opentelemetry.io/otel/baggage"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func executionMembers(member baggage.Member) bool {
	return slices.Contains(tracing.ExecutionBaggageKeys, member.Key())
}
