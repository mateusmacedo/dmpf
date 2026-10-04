package grpc_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
)

func TestEveryFailureIsRecordedUnderAnFND07CategoryOrOther(t *testing.T) {
	other := semconv.ErrorTypeOther.Value.AsString()
	cases := map[string]struct {
		err  error
		want string
	}{
		"success":             {nil, "ok"},
		"categorized":         {fmt.Errorf("wrapped: %w", categorizedFailure{}), "Unexpected"},
		"context deadline":    {fmt.Errorf("pq: secret: %w", context.DeadlineExceeded), "DeadlineExceeded"},
		"context cancelled":   {fmt.Errorf("pq: secret: %w", context.Canceled), "Cancelled"},
		"plain error":         {errors.New("pq: secret"), other},
		"INVALID_ARGUMENT":    {status.Error(codes.InvalidArgument, "x"), "Validation"},
		"FAILED_PRECONDITION": {status.Error(codes.FailedPrecondition, "x"), "DomainRejection"},
		"NOT_FOUND":           {status.Error(codes.NotFound, "x"), "NotFound"},
		"ABORTED":             {status.Error(codes.Aborted, "x"), "Conflict"},
		"ALREADY_EXISTS":      {status.Error(codes.AlreadyExists, "x"), "Conflict"},
		"PERMISSION_DENIED":   {status.Error(codes.PermissionDenied, "x"), "Forbidden"},
		"UNAUTHENTICATED":     {status.Error(codes.Unauthenticated, "x"), "Unauthenticated"},
		"UNAVAILABLE":         {fmt.Errorf("dial: %w", status.Error(codes.Unavailable, "x")), "TransientDependency"},
		"RESOURCE_EXHAUSTED":  {status.Error(codes.ResourceExhausted, "x"), "RateLimited"},
		"DEADLINE_EXCEEDED":   {status.Error(codes.DeadlineExceeded, "x"), "DeadlineExceeded"},
		"CANCELLED":           {status.Error(codes.Canceled, "x"), "Cancelled"},
		"INTERNAL":            {status.Error(codes.Internal, "x"), "Unexpected"},
		"UNKNOWN":             {status.Error(codes.Unknown, "x"), other},
		"OUT_OF_RANGE":        {status.Error(codes.OutOfRange, "x"), other},
		"UNIMPLEMENTED":       {status.Error(codes.Unimplemented, "x"), other},
		"DATA_LOSS":           {status.Error(codes.DataLoss, "x"), other},
		"undeclared code":     {status.Error(codes.Code(42), "x"), other},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := kernel.CategoryOf(c.err); got != c.want {
				t.Fatalf("CategoryOf(%v) = %q, want %q: the matrix of FND-07 §6.2 read back, or %s (RF-B1)", c.err, got, c.want, other)
			}
		})
	}
}
