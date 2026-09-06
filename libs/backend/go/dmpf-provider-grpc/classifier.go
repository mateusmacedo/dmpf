// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (GRP-08..10) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfgrpc

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/observe"
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

// categoryOf returns the bounded category under which a failure is recorded:
// the category a platform error carries, the context error, or the lowercase
// gRPC code — never the message (TRC-12, MET-07).
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
		return "deadline_exceeded"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	}
	return strings.ToLower(status.Code(err).String())
}
