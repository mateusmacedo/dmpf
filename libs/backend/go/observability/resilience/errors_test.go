package resilience_test

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func redacted(attr slog.Attr) map[string]string {
	if attr.Value.Kind() != slog.KindGroup {
		return map[string]string{attr.Key: attr.Value.String()}
	}
	members := map[string]string{}
	for _, member := range attr.Value.Group() {
		members[member.Key] = member.Value.String()
	}
	return members
}

func TestEveryDecoratorFailureReachesErrorTypeInTheVocabularyOfFND07(t *testing.T) {
	other := semconv.ErrorTypeOther.Value.AsString()
	cases := map[string]struct {
		err       error
		errorType string
		code      string
	}{
		"breaker open":                {resilience.ErrBreakerOpen, string(application.TransientDependency), "RES-12"},
		"bulkhead saturated":          {resilience.ErrBulkheadSaturated, string(application.TransientDependency), "RES-14"},
		"deadline exceeded":           {resilience.ErrDeadlineExceeded, string(application.DeadlineExceeded), "RES-06"},
		"cancelled":                   {resilience.ErrCancelled, string(application.Cancelled), "CTX-28"},
		"defer is the outbox":         {resilience.ErrDeferIsOutbox, other, "RES-37"},
		"retry around a unit of work": {resilience.ErrWrapsUnitOfWork, other, "RES-25"},
		"order without a reason":      {resilience.ErrOrderReasonRequired, other, "RES-22"},
		"blank field":                 {resilience.ErrBlankField, other, "RES-21"},
		"deadlines that do not fit":   {resilience.ErrDeadlineComposition, other, "RES-07"},
		"degraded answer": {
			&resilience.DegradedResult{Dependency: "payments", Cause: errors.New("dial tcp 10.0.0.7:443: connection refused")},
			other, "RES-37",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := redacted(redact.Error(fmt.Errorf("payments.Authorize: %w", tc.err)))

			if got[redact.KeyErrorType] != tc.errorType || got[tracing.KeyErrorCode] != tc.code {
				t.Fatalf("redact.Error() = %v, want %s=%q, a category of FND-07 or _OTHER, and the detail in %s=%q (RF-B1)",
					got, redact.KeyErrorType, tc.errorType, tracing.KeyErrorCode, tc.code)
			}
		})
	}
}
