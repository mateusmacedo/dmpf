package grpc_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type categorized struct {
	category string
	cause    error
}

func (c categorized) Error() string         { return c.category + ": " + c.cause.Error() }
func (c categorized) Unwrap() error         { return c.cause }
func (c categorized) ErrorCategory() string { return c.category }

func classifiedAs(category string) error {
	return categorized{category: category, cause: errors.New("secret")}
}

func TestStatusOfMapsEveryTechnicalOutcome(t *testing.T) {
	cases := map[string]struct {
		err     error
		want    codes.Code
		message string
	}{
		"absent aggregate":        {ports.ErrNotFound, codes.NotFound, "not found"},
		"classified as not found": {classifiedAs("NotFound"), codes.NotFound, "not found"},
		"version conflict":        {ports.ErrVersionConflict, codes.Aborted, "version conflict; replay the call"},
		"classified as conflict":  {classifiedAs("Conflict"), codes.Aborted, "version conflict; replay the call"},
		"wrapped conflict":        {fmt.Errorf("save: %w", ports.ErrVersionConflict), codes.Aborted, "version conflict; replay the call"},
		"deadline":                {context.DeadlineExceeded, codes.DeadlineExceeded, "deadline exceeded"},
		"wrapped deadline":        {fmt.Errorf("load: %w", context.DeadlineExceeded), codes.DeadlineExceeded, "deadline exceeded"},
		"cancellation":            {context.Canceled, codes.Canceled, "canceled"},
		"wrapped cancellation":    {fmt.Errorf("load: %w", context.Canceled), codes.Canceled, "canceled"},
		"anything else":           {errors.New("secret"), codes.Internal, "internal failure"},
		"unmapped category":       {classifiedAs("TransientDependency"), codes.Internal, "internal failure"},
		"nil":                     {nil, codes.Internal, "internal failure"},
		"reused key":              {fmt.Errorf("register: %w", ports.ErrIdempotencyMismatch), codes.FailedPrecondition, ""},
		"key in flight":           {fmt.Errorf("register: %w", ports.ErrIdempotencyInFlight), codes.Aborted, ""},
		"key absent":              {ports.ErrIdempotencyKeyAbsent, codes.InvalidArgument, ""},
		"key invalid":             {ports.ErrIdempotencyKeyInvalid, codes.InvalidArgument, ""},
		"already exists":          {fmt.Errorf("reserve: %w", ports.ErrAlreadyExists), codes.AlreadyExists, ""},

		// IDN-06 keeps the two apart: a subject that authenticated and lacks
		// what the operation needs must not be told it is unauthenticated.
		"denied authorization":          {ports.ErrDenied, codes.PermissionDenied, "permission denied"},
		"classified as forbidden":       {classifiedAs("Forbidden"), codes.PermissionDenied, "permission denied"},
		"wrapped denial":                {fmt.Errorf("authorize: %w", ports.ErrDenied), codes.PermissionDenied, "permission denied"},
		"credential absent":             {ports.ErrCredentialAbsent, codes.Unauthenticated, "unauthenticated"},
		"credential rejected":           {ports.ErrCredentialRejected, codes.Unauthenticated, "unauthenticated"},
		"subject unresolved":            {ports.ErrSubjectUnresolved, codes.Unauthenticated, "unauthenticated"},
		"classified as unauthenticated": {classifiedAs("Unauthenticated"), codes.Unauthenticated, "unauthenticated"},

		"category above another sentinel": {categorized{category: "Conflict", cause: ports.ErrNotFound}, codes.NotFound, "not found"},
		"idempotency above a category":    {categorized{category: "NotFound", cause: ports.ErrIdempotencyInFlight}, codes.Aborted, ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := kernelgrpc.StatusOf(tc.err)
			if got := status.Code(err); got != tc.want {
				t.Fatalf("StatusOf(%v) = %v, want %v", tc.err, got, tc.want)
			}
			message := status.Convert(err).Message()
			if strings.Contains(message, "secret") {
				t.Fatalf("message = %q, want no internal detail (ERR-20)", message)
			}
			if tc.message != "" && message != tc.message {
				t.Fatalf("message = %q, want %q", message, tc.message)
			}
		})
	}
}
