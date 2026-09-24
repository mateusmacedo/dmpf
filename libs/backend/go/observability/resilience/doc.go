// Package resilience holds the resilience sheet of an operation, the route
// validation of the derived deadline and the decorators — timeout, circuit
// breaker, bulkhead, degradation and retry — composed in the canonical order.
//
// A decorator never wraps a unit of work: repeating a transaction is the caller's
// decision, not a policy this package may take on its own.
package resilience
