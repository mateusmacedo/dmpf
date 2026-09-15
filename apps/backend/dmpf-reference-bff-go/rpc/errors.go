package rpc

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dmpfgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/deadline"
)

// Failure is the public face of a failed call: an HTTP status, a stable code
// and a message that never carries what the context or the transport said.
type Failure struct {
	Status  int
	Code    string
	Message string
}

var (
	internalFailure = Failure{http.StatusInternalServerError, "internal-failure", "the request could not be completed"}
	deadlineFailure = Failure{http.StatusGatewayTimeout, "deadline-exceeded", "the deadline of the request expired before the context answered"}

	// codeNames pairs each code a context may answer with the stable code the
	// edge publishes. The HTTP status is never written here: it comes from
	// dmpfgrpc.HTTPStatus, the one table GRP-14 fixes.
	codeNames = map[codes.Code]struct{ code, message string }{
		codes.NotFound:           {"not-found", "no resource with this id"},
		codes.Aborted:            {"version-conflict", "a concurrent write advanced the resource; replay the request"},
		codes.AlreadyExists:      {"already-exists", "the resource already exists"},
		codes.DeadlineExceeded:   {deadlineFailure.Code, deadlineFailure.Message},
		codes.Unavailable:        {"unavailable", "the context is unavailable"},
		codes.ResourceExhausted:  {"admission-refused", "the context refused the call; retry later"},
		codes.InvalidArgument:    {"invalid-request", "the context refused the request as invalid"},
		codes.FailedPrecondition: {"failed-precondition", "the resource is not in a state that accepts this request"},
		codes.OutOfRange:         {"out-of-range", "a value of the request is outside the accepted range"},
		codes.PermissionDenied:   {"permission-denied", "the caller may not perform this operation"},
		codes.Unauthenticated:    {"unauthenticated", "the caller is not authenticated"},
		codes.Unimplemented:      {"unimplemented", "the context does not implement this operation"},
		codes.Canceled:           {"client-closed-request", "the caller went away before the context answered"},
	}
)

// Classify reads the local deadline errors before status.Code, which would
// report them as Unknown and turn a timeout into a 500.
func Classify(err error) Failure {
	switch {
	case errors.Is(err, deadline.ErrNoDeadline):
		return internalFailure
	case errors.Is(err, deadline.ErrDeadlineExhausted), errors.Is(err, context.DeadlineExceeded):
		return deadlineFailure
	}
	s, isStatus := status.FromError(err)
	if !isStatus {
		return internalFailure
	}
	named, mapped := codeNames[s.Code()]
	if !mapped {
		return internalFailure
	}
	return Failure{dmpfgrpc.HTTPStatus(s.Code()), named.code, named.message}
}
