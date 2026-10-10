package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Outcome is matched by shape: this provider block cannot import the
// application block that declares usecase.Outcome (DMPF-D001, ADR-061).
type Outcome[R any] interface {
	Response() R
	Rejection() (*domain.Rejection, bool)
}

// WHY: the absence of what the interceptor mounted is a wiring defect of this
// server, never a fault of the caller, so it answers Internal rather than
// leaving the handler to invent a context (CTX-03).
func ExecutionOf(ctx context.Context) (ports.ExecutionContext, error) {
	execution, ok := ports.ExecutionContextFrom(ctx)
	if !ok {
		return ports.ExecutionContext{}, status.Error(codes.Internal, "the execution context was not assembled")
	}
	return execution, nil
}

func Command[R, Resp any, O Outcome[R]](ctx context.Context, run func(context.Context) (O, error),
	refused func(*domain.Rejection) Resp, accepted func(R) Resp) (Resp, error) {
	var none Resp
	if _, err := ExecutionOf(ctx); err != nil {
		return none, err
	}
	out, err := run(ctx)
	if err != nil {
		return none, StatusOf(err)
	}
	if rejection, declined := out.Rejection(); declined {
		return refused(rejection), nil
	}
	return accepted(out.Response()), nil
}
