package app_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestStatusOfReadsEveryCategoryOfTheApplicationFailure(t *testing.T) {
	want := map[application.Category]codes.Code{
		application.Validation:          codes.Internal,
		application.DomainRejection:     codes.Internal,
		application.NotFound:            codes.NotFound,
		application.Conflict:            codes.Aborted,
		application.Forbidden:           codes.PermissionDenied,
		application.Unauthenticated:     codes.Unauthenticated,
		application.TransientDependency: codes.Internal,
		application.RateLimited:         codes.Internal,
		application.DeadlineExceeded:    codes.Internal,
		application.Cancelled:           codes.Internal,
		application.Unexpected:          codes.Internal,
	}

	for category, code := range want {
		t.Run(string(category), func(t *testing.T) {
			err := kernelgrpc.StatusOf(application.NewFailure(category, false, errors.New("secret")))
			if got := status.Code(err); got != code {
				t.Fatalf("StatusOf(Failure(%s)) = %v, want %v", category, got, code)
			}
		})
	}
}

func TestStatusOfLetsTheLowestRowWinOverTheCategoryOfAFailure(t *testing.T) {
	cases := map[string]struct {
		err  error
		want codes.Code
	}{
		"conflict around not found":         {application.NewFailure(application.Conflict, true, ports.ErrNotFound), codes.NotFound},
		"not found around version conflict": {application.NewFailure(application.NotFound, false, ports.ErrVersionConflict), codes.NotFound},
		"forbidden around deadline":         {application.NewFailure(application.Forbidden, false, context.DeadlineExceeded), codes.DeadlineExceeded},
		"unauthenticated around denial":     {application.NewFailure(application.Unauthenticated, false, ports.ErrDenied), codes.PermissionDenied},
		"forbidden around reused key":       {application.NewFailure(application.Forbidden, false, ports.ErrIdempotencyMismatch), codes.FailedPrecondition},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := status.Code(kernelgrpc.StatusOf(tc.err)); got != tc.want {
				t.Fatalf("StatusOf(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
