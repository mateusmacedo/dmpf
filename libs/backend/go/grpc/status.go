// comment-discipline-ok-file: arquivo de contrato público; o godoc cita a regra de FND-06 (GRP-14) que o símbolo realiza, dentro do limite de 3 linhas.

package grpc

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// StatusClientClosedRequest is the non-standard 499 the ecosystem uses for a
// cancelled call (GRP-14).
const StatusClientClosedRequest = 499

// HTTPStatus is the canonical mapping of a gRPC code to HTTP (GRP-14): the ten
// codes that FND-06 fixes, the rest as grpc-gateway maps them, and 500 for a
// code outside the table.
func HTTPStatus(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return StatusClientClosedRequest
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

const (
	categoryNotFound        = "NotFound"
	categoryConflict        = "Conflict"
	categoryForbidden       = "Forbidden"
	categoryUnauthenticated = "Unauthenticated"
)

// StatusOf maps the technical channel of a use case to a status whose message
// carries no internal detail (ERR-20). The first matching row decides, and the
// category is read off the first node of the chain that declares one.
func StatusOf(err error) error {
	if mapped, ok := IdempotencyStatus(err); ok {
		return mapped
	}
	var category string
	var categorized interface{ ErrorCategory() string }
	if errors.As(err, &categorized) {
		category = categorized.ErrorCategory()
	}
	switch {
	case errors.Is(err, ports.ErrNotFound), category == categoryNotFound:
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, ports.ErrVersionConflict), category == categoryConflict:
		return status.Error(codes.Aborted, "version conflict; replay the call")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "canceled")
	case errors.Is(err, ports.ErrDenied), category == categoryForbidden:
		return status.Error(codes.PermissionDenied, "permission denied")
	case errors.Is(err, ports.ErrCredentialAbsent), errors.Is(err, ports.ErrCredentialRejected),
		errors.Is(err, ports.ErrSubjectUnresolved), category == categoryUnauthenticated:
		return status.Error(codes.Unauthenticated, "unauthenticated")
	default:
		return status.Error(codes.Internal, "internal failure")
	}
}
