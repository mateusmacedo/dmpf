package tracing

import (
	"context"

	"go.opentelemetry.io/otel/baggage"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ExecutionBaggageKeys are the members the baggage processors copy onto every
// span and log record of the process (RF-B8). The propagator never injects the
// baggage, so they stay in-process.
var ExecutionBaggageKeys = []string{KeyCorrelationID, KeyRequestID, KeyTenantID}

// WithExecutionBaggage puts the identifiers of the execution in the baggage of
// ctx. A span opened before it does not receive them.
func WithExecutionBaggage(ctx context.Context, execution ports.ExecutionContext) context.Context {
	bag := baggage.FromContext(ctx)
	for _, kv := range ExecutionAttributes(execution).KeyValues() {
		member, err := baggage.NewMemberRaw(string(kv.Key), kv.Value.AsString())
		if err != nil {
			continue
		}
		if next, err := bag.SetMember(member); err == nil {
			bag = next
		}
	}
	return baggage.ContextWithBaggage(ctx, bag)
}
