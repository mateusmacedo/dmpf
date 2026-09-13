// comment-discipline-ok-file: arquivo de contrato público; o godoc cita a regra de FND-06 (GRP-14) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfgrpc

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

// StatusClientClosedRequest is the non-standard 499 the ecosystem uses for a
// cancelled call (GRP-14).
const StatusClientClosedRequest = 499

// HTTPStatus is the canonical mapping of a gRPC code to HTTP (GRP-14): the ten
// codes that FND-06 fixes, the rest as grpc-gateway maps them, and 500 for a
// code outside the table.
func HTTPStatus(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return StatusClientClosedRequest
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
