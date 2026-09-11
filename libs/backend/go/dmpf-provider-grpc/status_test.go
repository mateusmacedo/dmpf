package dmpfgrpc_test

import (
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"

	dmpfgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-grpc"
)

func TestHTTPStatusFollowsTheCanonicalTable(t *testing.T) {
	want := map[codes.Code]int{
		codes.OK:                 http.StatusOK,
		codes.InvalidArgument:    http.StatusBadRequest,
		codes.Unauthenticated:    http.StatusUnauthorized,
		codes.PermissionDenied:   http.StatusForbidden,
		codes.NotFound:           http.StatusNotFound,
		codes.AlreadyExists:      http.StatusConflict,
		codes.ResourceExhausted:  http.StatusTooManyRequests,
		codes.Unavailable:        http.StatusServiceUnavailable,
		codes.DeadlineExceeded:   http.StatusGatewayTimeout,
		codes.Canceled:           dmpfgrpc.StatusClientClosedRequest,
		codes.Unknown:            http.StatusInternalServerError,
		codes.FailedPrecondition: http.StatusBadRequest,
		codes.Aborted:            http.StatusConflict,
		codes.OutOfRange:         http.StatusBadRequest,
		codes.Unimplemented:      http.StatusNotImplemented,
		codes.Internal:           http.StatusInternalServerError,
		codes.DataLoss:           http.StatusInternalServerError,
		codes.Code(999):          http.StatusInternalServerError,
	}
	for code, status := range want {
		if got := dmpfgrpc.HTTPStatus(code); got != status {
			t.Errorf("HTTPStatus(%v) = %d, want %d (GRP-14)", code, got, status)
		}
	}
}
