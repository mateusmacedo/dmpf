package grpc_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestIdempotencyStatusMapsEveryOutcomeToItsCodeAndReason(t *testing.T) {
	cases := []struct {
		err    error
		code   codes.Code
		reason string
	}{
		{ports.ErrIdempotencyMismatch, codes.FailedPrecondition, kernelgrpc.ReasonReusedIdempotencyKey},
		{ports.ErrIdempotencyInFlight, codes.Aborted, kernelgrpc.ReasonInFlightIdempotencyKey},
		{ports.ErrIdempotencyKeyAbsent, codes.InvalidArgument, kernelgrpc.ReasonMissingIdempotencyKey},
		{ports.ErrIdempotencyKeyInvalid, codes.InvalidArgument, kernelgrpc.ReasonInvalidIdempotencyKey},
		{ports.ErrAlreadyExists, codes.AlreadyExists, ""},
	}
	for _, c := range cases {
		t.Run(c.err.Error(), func(t *testing.T) {
			mapped, ok := kernelgrpc.IdempotencyStatus(fmt.Errorf("application: add item: %w", c.err))
			if !ok {
				t.Fatalf("IdempotencyStatus(%v) was not recognised", c.err)
			}
			if got := status.Code(mapped); got != c.code {
				t.Fatalf("code = %v, want %v", got, c.code)
			}
			if got := kernelgrpc.ReasonOf(mapped); got != c.reason {
				t.Fatalf("ReasonOf() = %q, want %q", got, c.reason)
			}
			if strings.Contains(strings.ToLower(status.Convert(mapped).Message()), "replay the") {
				t.Fatalf("message %q tells the client to replay: only in-flight converges on a retry, and it says so with the same key", status.Convert(mapped).Message())
			}
		})
	}
}

func TestIdempotencyStatusLeavesOtherFailuresToTheCaller(t *testing.T) {
	for _, err := range []error{ports.ErrVersionConflict, ports.ErrNotFound, errors.New("grpc_test: anything else")} {
		if mapped, ok := kernelgrpc.IdempotencyStatus(err); ok {
			t.Errorf("IdempotencyStatus(%v) = %v, true; want false", err, mapped)
		}
	}
}

func TestKeyStatusCarriesTheReasonOfTheEdgeRefusal(t *testing.T) {
	for _, reason := range []string{kernelgrpc.ReasonMissingIdempotencyKey, kernelgrpc.ReasonInvalidIdempotencyKey} {
		err := kernelgrpc.KeyStatus(reason)
		if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != reason {
			t.Errorf("KeyStatus(%s) = (%v, %q), want (InvalidArgument, %s)", reason, status.Code(err), kernelgrpc.ReasonOf(err), reason)
		}
	}
}

func TestReasonOfIsEmptyWithoutAnErrorInfo(t *testing.T) {
	for _, err := range []error{nil, errors.New("grpc_test: plain"), status.Error(codes.Aborted, "version conflict")} {
		if got := kernelgrpc.ReasonOf(err); got != "" {
			t.Errorf("ReasonOf(%v) = %q, want empty", err, got)
		}
	}
}
