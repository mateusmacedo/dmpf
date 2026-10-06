// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (GRP-08..10) que o símbolo realiza, dentro do limite de 3 linhas.

package grpc

import (
	"context"
	"errors"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"
)

// StatusClassifier says whether a failed call is worth another attempt: only a
// status whose code is in the method's declared transient list is (GRP-09).
// Everything else — another code, a refusal of ours, a plain error — is not.
func StatusClassifier(retryable []codes.Code) retry.Classifier {
	declared := make(map[codes.Code]struct{}, len(retryable))
	for _, code := range retryable {
		declared[code] = struct{}{}
	}
	return func(err error) retry.Retryability {
		s, isStatus := status.FromError(err)
		if !isStatus {
			return retry.NotRetryable
		}
		if _, transient := declared[s.Code()]; transient {
			return retry.Retryable
		}
		return retry.NotRetryable
	}
}

// dependencyFailure is what the breaker holds against the dependency:
// unavailability or a transport error, never an answer such as NOT_FOUND
// (RES-10, RES-12).
func dependencyFailure(err error) bool {
	s, isStatus := status.FromError(err)
	if !isStatus {
		return true
	}
	switch s.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Internal, codes.Unknown, codes.DataLoss:
		return true
	default:
		return false
	}
}

// categoryOf returns the bounded category under which a failure is recorded:
// the category a platform error carries, or the FND-07 category its code or
// context error maps to (§6.2), `_OTHER` outside it — never the message (TRC-12).
func categoryOf(err error) string {
	if err == nil {
		return observe.CategoryOK
	}
	var categorized interface{ ErrorCategory() string }
	if errors.As(err, &categorized) {
		return categorized.ErrorCategory()
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return categoryDeadlineExceeded
	case errors.Is(err, context.Canceled):
		return categoryCancelled
	}
	if category, mapped := fnd07Categories[status.Code(err)]; mapped {
		return category
	}
	return semconv.ErrorTypeOther.Value.AsString()
}

const (
	categoryDeadlineExceeded = "DeadlineExceeded"
	categoryCancelled        = "Cancelled"
)

var fnd07Categories = map[codes.Code]string{
	codes.InvalidArgument: "Validation", codes.FailedPrecondition: "DomainRejection", codes.NotFound: "NotFound",
	codes.Aborted: "Conflict", codes.AlreadyExists: "Conflict", codes.PermissionDenied: "Forbidden",
	codes.Unauthenticated: "Unauthenticated", codes.Unavailable: "TransientDependency", codes.ResourceExhausted: "RateLimited",
	codes.DeadlineExceeded: categoryDeadlineExceeded, codes.Canceled: categoryCancelled, codes.Internal: "Unexpected",
}
