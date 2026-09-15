package rpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// statusOf maps the technical channel to a gRPC status whose message carries
// no internal detail (ERR-20); a classified failure keeps its code when wrapped.
func statusOf(err error) error {
	failure, classified := errors.AsType[*dmpfapplication.Failure](err)
	switch {
	case errors.Is(err, dmpfports.ErrNotFound), classified && failure.Category() == dmpfapplication.NotFound:
		return status.Error(codes.NotFound, "not found")
	case errors.Is(err, dmpfports.ErrVersionConflict), classified && failure.Category() == dmpfapplication.Conflict:
		return status.Error(codes.Aborted, "version conflict; replay the call")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "canceled")
	default:
		return status.Error(codes.Internal, "internal failure")
	}
}
