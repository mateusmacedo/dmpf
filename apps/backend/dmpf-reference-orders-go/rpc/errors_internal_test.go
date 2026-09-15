package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

func TestStatusOfMapsEveryTechnicalOutcome(t *testing.T) {
	cases := map[string]struct {
		err  error
		want codes.Code
	}{
		"absent aggregate":        {dmpfports.ErrNotFound, codes.NotFound},
		"classified as not found": {dmpfapplication.NewFailure(dmpfapplication.NotFound, false, errors.New("secret")), codes.NotFound},
		"version conflict":        {dmpfports.ErrVersionConflict, codes.Aborted},
		"classified as conflict":  {dmpfapplication.NewFailure(dmpfapplication.Conflict, true, errors.New("secret")), codes.Aborted},
		"wrapped conflict":        {fmt.Errorf("save: %w", dmpfports.ErrVersionConflict), codes.Aborted},
		"deadline":                {context.DeadlineExceeded, codes.DeadlineExceeded},
		"cancellation":            {context.Canceled, codes.Canceled},
		"anything else":           {errors.New("secret"), codes.Internal},
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
