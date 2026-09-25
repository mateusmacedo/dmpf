package rpc_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

func TestClassifyMapsFailuresWithoutInternalDetail(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", status.Error(codes.NotFound, "row absent in dmpf_example_orders"), http.StatusNotFound, "not-found"},
		{"version conflict", status.Error(codes.Aborted, "version 3"), http.StatusConflict, "version-conflict"},
		{"remote deadline", status.Error(codes.DeadlineExceeded, "late"), http.StatusGatewayTimeout, "deadline-exceeded"},
		{"local context deadline", fmt.Errorf("call: %w", context.DeadlineExceeded), http.StatusGatewayTimeout, "deadline-exceeded"},
		{"local exhausted deadline", fmt.Errorf("%w: orders", deadline.ErrDeadlineExhausted), http.StatusGatewayTimeout, "deadline-exceeded"},
		{"missing deadline", deadline.ErrNoDeadline, http.StatusInternalServerError, "internal-failure"},
		{"unavailable", status.Error(codes.Unavailable, "dial tcp"), http.StatusServiceUnavailable, "unavailable"},
		{"admission", status.Error(codes.ResourceExhausted, "limit"), http.StatusTooManyRequests, "admission-refused"},
		{"invalid argument", status.Error(codes.InvalidArgument, "order_id"), http.StatusBadRequest, "invalid-request"},
		{"internal", status.Error(codes.Internal, "panic: nil map"), http.StatusInternalServerError, "internal-failure"},
		{"plain error", errors.New("pgx: connection refused"), http.StatusInternalServerError, "internal-failure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rpc.Classify(tc.err)
			if got.Status != tc.status || got.Code != tc.code {
				t.Fatalf("Classify() = %+v, want %d %q", got, tc.status, tc.code)
			}
			if got.Message == "" {
				t.Fatal("Classify() carries no public message")
			}
		})
	}
}

func TestEveryCodeAContextEmitsKeepsItsCanonicalStatus(t *testing.T) {
	// WHY: the kernel already fixes the GRP-14 table; a code missing from the
	// edge's own map used to collapse into 500, so a client disconnect
	// (Canceled) was reported as a server fault.
	for _, tc := range []struct {
		code codes.Code
		want int
	}{
		{codes.Canceled, grpc.StatusClientClosedRequest},
		{codes.PermissionDenied, http.StatusForbidden},
		{codes.Unauthenticated, http.StatusUnauthorized},
		{codes.Unimplemented, http.StatusNotImplemented},
		{codes.FailedPrecondition, http.StatusBadRequest},
		{codes.OutOfRange, http.StatusBadRequest},
		{codes.AlreadyExists, http.StatusConflict},
		{codes.NotFound, http.StatusNotFound},
		{codes.Aborted, http.StatusConflict},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.ResourceExhausted, http.StatusTooManyRequests},
		{codes.InvalidArgument, http.StatusBadRequest},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout},
	} {
		t.Run(tc.code.String(), func(t *testing.T) {
			got := rpc.Classify(status.Error(tc.code, "from the context"))
			if got.Status != tc.want {
				t.Fatalf("Classify(%s).Status = %d, want %d (code %q)", tc.code, got.Status, tc.want, got.Code)
			}
			if got.Code == "" {
				t.Fatalf("Classify(%s).Code is empty; every answer needs a stable code", tc.code)
			}
		})
	}
}

func TestAnUnknownCodeStaysInternal(t *testing.T) {
	got := rpc.Classify(status.Error(codes.Unknown, "surprise"))
	if got.Status != http.StatusInternalServerError {
		t.Fatalf("Classify(Unknown).Status = %d, want 500", got.Status)
	}
}
