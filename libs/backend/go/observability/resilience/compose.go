package resilience

import (
	"fmt"
	"slices"
)

// Names of the eight decorator positions of RES-22. The ninth position of the
// canonical order is the call itself, which is not a decorator.
const (
	SlotTracing   = "tracing"
	SlotMetrics   = "metrics"
	SlotLogging   = "logging"
	SlotBulkhead  = "bulkhead"
	SlotBreaker   = "breaker"
	SlotRateLimit = "rate_limit"
	SlotRetry     = "retry"
	SlotTimeout   = "timeout"
)

// CanonicalOrder is the order of RES-22, from the outside in. Tracing is
// outermost so a refusal by any decorator below still lands on the span, and
// timeout is innermost so the deadline bounds the call and not the queueing.
func CanonicalOrder() []string {
	return []string{
		SlotTracing, SlotMetrics, SlotLogging, SlotBulkhead,
		SlotBreaker, SlotRateLimit, SlotRetry, SlotTimeout,
	}
}

// Slots are the decorators of each position. An empty position is allowed only
// when the sheet declares the corresponding field not applicable.
type Slots struct {
	Tracing   Decorator
	Metrics   Decorator
	Logging   Decorator
	Bulkhead  Decorator
	Breaker   Decorator
	RateLimit Decorator
	Retry     Decorator
	Timeout   Decorator
}

func (s Slots) at(position string) Decorator {
	switch position {
	case SlotTracing:
		return s.Tracing
	case SlotMetrics:
		return s.Metrics
	case SlotLogging:
		return s.Logging
	case SlotBulkhead:
		return s.Bulkhead
	case SlotBreaker:
		return s.Breaker
	case SlotRateLimit:
		return s.RateLimit
	case SlotRetry:
		return s.Retry
	case SlotTimeout:
		return s.Timeout
	default:
		return nil
	}
}

// Compose applies the decorators in the canonical order of RES-22.
func Compose(sheet Sheet, slots Slots) (Call, error) {
	return build(sheet, slots, CanonicalOrder())
}

// ComposeWithOrder applies them in another order, which RES-22 allows only with
// a declared reason: an order nobody justified is an order nobody reviewed.
func ComposeWithOrder(sheet Sheet, slots Slots, order []string, reason string) (Call, error) {
	if reason == "" {
		return nil, fmt.Errorf("%w: %s", ErrOrderReasonRequired, sheet.Dependency)
	}
	if err := validateOrder(order); err != nil {
		return nil, err
	}
	return build(sheet, slots, order)
}

// validateOrder refuses an order that is not a permutation of the canonical
// one: a missing position would drop a decorator in silence, and a repeated one
// would apply it twice.
func validateOrder(order []string) error {
	canonical := CanonicalOrder()
	if len(order) != len(canonical) {
		return fmt.Errorf("%w: the order has %d positions, want the %d of RES-22", ErrBlankField, len(order), len(canonical))
	}

	seen := make(map[string]bool, len(order))
	for _, position := range order {
		if !slices.Contains(canonical, position) {
			return fmt.Errorf("%w: %q is not a position of RES-22", ErrBlankField, position)
		}
		if seen[position] {
			return fmt.Errorf("%w: %q appears twice in the order", ErrBlankField, position)
		}
		seen[position] = true
	}
	return nil
}

// build wraps the call from the inside out, so the first position of the order
// ends up outermost.
func build(sheet Sheet, slots Slots, order []string) (Call, error) {
	if err := sheet.Validate(); err != nil {
		return nil, err
	}
	if err := requireDeclaredAbsences(sheet, slots); err != nil {
		return nil, err
	}

	call := Call(Direct)
	for _, position := range slices.Backward(order) {
		if decorator := slots.at(position); decorator != nil {
			call = decorator(call)
		}
	}
	return call, nil
}

// requireDeclaredAbsences refuses an empty position the sheet did not declare
// not applicable (RES-21). Rate limiting and cache are the positions this
// delivery leaves empty, and the sheet says so with a reason; a position left
// empty by oversight is caught here.
func requireDeclaredAbsences(sheet Sheet, slots Slots) error {
	absences := map[string]bool{
		SlotTracing:   slots.Tracing == nil,
		SlotMetrics:   slots.Metrics == nil,
		SlotLogging:   slots.Logging == nil,
		SlotBulkhead:  slots.Bulkhead == nil,
		SlotBreaker:   slots.Breaker == nil,
		SlotRateLimit: slots.RateLimit == nil,
		SlotRetry:     slots.Retry == nil,
		SlotTimeout:   slots.Timeout == nil,
	}

	declared := map[string]bool{
		SlotBulkhead:  isSkipped(sheet.Bulkhead),
		SlotBreaker:   isSkipped(sheet.Breaker),
		SlotRateLimit: isSkipped(sheet.RateLimit),
		SlotRetry:     isSkipped(sheet.Retry),
		SlotTimeout:   isSkipped(sheet.Deadline),
	}

	missing := make([]string, 0, len(absences))
	for position, empty := range absences {
		if !empty {
			continue
		}
		// Tracing, metrics and logging have no field of their own in the sheet:
		// RES-23 makes every decorator observable, so their absence is never a
		// declared one.
		if reason, hasField := declared[position]; !hasField || !reason {
			missing = append(missing, position)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return fmt.Errorf("%w: %s: %v are empty and the sheet does not declare them not applicable",
			ErrBlankField, sheet.Dependency, missing)
	}
	return nil
}

func isSkipped[T any](field Field[T]) bool {
	_, skipped := field.Skipped()
	return skipped
}
