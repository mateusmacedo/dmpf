package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

var (
	ErrInvalidOutcomeTable = errors.New("http: outcome table is invalid")

	ErrOutboundKeyRequired = errors.New("http: outbound key is empty or the route declares no idempotency key header")

	// ErrOutboundKeyHeaderReplayable: with this header the http.Transport resends a
	// written POST on a dead keep-alive by itself (net/http/request.go:1570, go1.27.1).
	ErrOutboundKeyHeaderReplayable = errors.New("http: outbound key header is Idempotency-Key or X-Idempotency-Key")
)

type OutcomeTable struct {
	Version string
	Status  map[int]ports.ExternalClass
}

func (t OutcomeTable) Validate() error {
	if t.Version == "" {
		return fmt.Errorf("%w: version", ErrInvalidOutcomeTable)
	}
	for status, class := range t.Status {
		if status < 100 || status > 599 {
			return fmt.Errorf("%w: %s: status %d", ErrInvalidOutcomeTable, t.Version, status)
		}
		if class < ports.ClassAccepted || class > ports.ClassSchemaMismatch {
			return fmt.Errorf("%w: %s: status %d maps to class %d", ErrInvalidOutcomeTable, t.Version, status, class)
		}
	}
	return nil
}

// Classify takes as unsent only a failed dial or a refusal before any attempt:
// WroteRequest may fire after Do returns (net/http/request.go, Request.write), so
// no other network error, cancellation or deadline proves the partner got nothing.
func (t OutcomeTable) Classify(resp *http.Response, err error) ports.ExternalClass {
	if err != nil {
		var transient *RetryableStatusError
		if errors.As(err, &transient) {
			return t.byStatus(transient.Status)
		}
		var unsent *notSentError
		if errors.As(err, &unsent) {
			return classifyUnsent(err)
		}
		var op *net.OpError
		if errors.As(err, &op) && op.Op == "dial" {
			return ports.ClassRetryableTransport
		}
		return ports.ClassUnknownSent
	}
	if resp == nil {
		return ports.ClassUnknownAmbiguous
	}
	return t.byStatus(resp.StatusCode)
}

func classifyUnsent(err error) ports.ExternalClass {
	switch {
	case errors.Is(err, ErrRouteNotDeclared), errors.Is(err, ErrMethodMismatch),
		errors.Is(err, ErrBodyNotReplayable), errors.Is(err, deadline.ErrNoDeadline):
		return ports.ClassFinalValidation
	case errors.Is(err, resilience.ErrBreakerOpen), errors.Is(err, resilience.ErrBulkheadSaturated),
		errors.Is(err, deadline.ErrDeadlineExhausted), errors.Is(err, resilience.ErrDeadlineExceeded):
		return ports.ClassRetryableTransport
	default:
		return ports.ClassUnknownSent
	}
}

type notSentError struct{ err error }

func (e *notSentError) Error() string { return e.err.Error() }

func (e *notSentError) Unwrap() error { return e.err }

func (t OutcomeTable) byStatus(status int) ports.ExternalClass {
	if class, declared := t.Status[status]; declared {
		return class
	}
	return ports.ClassUnknownAmbiguous
}

type outboundKeyedKey struct{}

// WithOutboundKey sets the key in the route's header and marks the request so
// Do sends it in one attempt: retrying is the kernel's decision, by the class.
func (r Route) WithOutboundKey(req *http.Request, key ports.OutboundKey) (*http.Request, error) {
	if r.IdempotencyKey == "" || key == "" {
		return nil, fmt.Errorf("%w: %s", ErrOutboundKeyRequired, r.Name)
	}
	switch http.CanonicalHeaderKey(r.IdempotencyKey) {
	case "Idempotency-Key", "X-Idempotency-Key":
		return nil, fmt.Errorf("%w: %s: %s", ErrOutboundKeyHeaderReplayable, r.Name, r.IdempotencyKey)
	}
	keyed := req.Clone(context.WithValue(req.Context(), outboundKeyedKey{}, true))
	keyed.Header.Set(r.IdempotencyKey, string(key))
	return keyed, nil
}

func outboundKeyed(req *http.Request) bool {
	keyed, _ := req.Context().Value(outboundKeyedKey{}).(bool)
	return keyed
}
