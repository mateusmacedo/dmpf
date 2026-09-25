package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestStatusOfMapsEveryTechnicalOutcome(t *testing.T) {
	cases := map[string]struct {
		err  error
		want codes.Code
	}{
		"absent aggregate":        {ports.ErrNotFound, codes.NotFound},
		"classified as not found": {application.NewFailure(application.NotFound, false, errors.New("secret")), codes.NotFound},
		"version conflict":        {ports.ErrVersionConflict, codes.Aborted},
		"classified as conflict":  {application.NewFailure(application.Conflict, true, errors.New("secret")), codes.Aborted},
		"wrapped conflict":        {fmt.Errorf("save: %w", ports.ErrVersionConflict), codes.Aborted},
		"deadline":                {context.DeadlineExceeded, codes.DeadlineExceeded},
		"cancellation":            {context.Canceled, codes.Canceled},
		"anything else":           {errors.New("secret"), codes.Internal},

		// IDN-06 keeps the two apart: a subject that authenticated and lacks
		// what the operation needs must not be told it is unauthenticated.
		"denied authorization":          {ports.ErrDenied, codes.PermissionDenied},
		"classified as forbidden":       {application.NewFailure(application.Forbidden, false, errors.New("secret")), codes.PermissionDenied},
		"wrapped denial":                {fmt.Errorf("authorize: %w", ports.ErrDenied), codes.PermissionDenied},
		"credential absent":             {ports.ErrCredentialAbsent, codes.Unauthenticated},
		"credential rejected":           {ports.ErrCredentialRejected, codes.Unauthenticated},
		"subject unresolved":            {ports.ErrSubjectUnresolved, codes.Unauthenticated},
		"classified as unauthenticated": {application.NewFailure(application.Unauthenticated, false, errors.New("secret")), codes.Unauthenticated},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := statusOf(tc.err)
			if got := status.Code(err); got != tc.want {
				t.Fatalf("statusOf(%v) = %v, want %v", tc.err, got, tc.want)
			}
			if message := status.Convert(err).Message(); strings.Contains(message, "secret") {
				t.Fatalf("message = %q, want no internal detail (ERR-20)", message)
			}
		})
	}
}
