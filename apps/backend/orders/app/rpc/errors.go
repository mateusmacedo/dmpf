package rpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// statusOf maps the technical channel to a gRPC status whose message carries
// no internal detail (ERR-20); a classified failure keeps its code when wrapped.
func statusOf(err error) error {
	failure, classified := errors.AsType[*application.Failure](err)
	switch {
	case errors.Is(err, ports.ErrNotFound), classified && failure.Category() == application.NotFound:
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, ports.ErrVersionConflict), classified && failure.Category() == application.Conflict:
		return status.Error(codes.Aborted, "version conflict; replay the call")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "canceled")
	case errors.Is(err, ports.ErrDenied), classified && failure.Category() == application.Forbidden:
		return status.Error(codes.PermissionDenied, "permission denied")
	case errors.Is(err, ports.ErrCredentialAbsent), errors.Is(err, ports.ErrCredentialRejected),
		errors.Is(err, ports.ErrSubjectUnresolved), classified && failure.Category() == application.Unauthenticated:
		return status.Error(codes.Unauthenticated, "unauthenticated")
	default:
		return status.Error(codes.Internal, "internal failure")
	}
}
