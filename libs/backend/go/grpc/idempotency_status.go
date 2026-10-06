package grpc

import (
	"errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// The reasons a status carries in google.rpc.ErrorInfo, stable on the wire:
// the BFF tells the idempotency refusals apart by them, not by the message.
const (
	ReasonMissingIdempotencyKey  = "MISSING_IDEMPOTENCY_KEY"
	ReasonInvalidIdempotencyKey  = "INVALID_IDEMPOTENCY_KEY"
	ReasonReusedIdempotencyKey   = "REUSED_IDEMPOTENCY_KEY"
	ReasonInFlightIdempotencyKey = "IN_FLIGHT_IDEMPOTENCY_KEY"

	errorInfoDomain = "dmpf"
)

// IdempotencyStatus maps the idempotency outcomes a use case reports to their
// status; ok is false for any other error, which the context maps itself. No
// message tells the client to replay except in-flight, the one that converges.
func IdempotencyStatus(err error) (mapped error, ok bool) {
	switch {
	case errors.Is(err, ports.ErrIdempotencyMismatch):
		return withReason(codes.FailedPrecondition, "idempotency key already used by a different request", ReasonReusedIdempotencyKey), true
	case errors.Is(err, ports.ErrIdempotencyInFlight):
		return withReason(codes.Aborted, "a request with this idempotency key is in flight; repeat it with the same key", ReasonInFlightIdempotencyKey), true
	case errors.Is(err, ports.ErrIdempotencyKeyAbsent):
		return withReason(codes.InvalidArgument, "idempotency key required", ReasonMissingIdempotencyKey), true
	case errors.Is(err, ports.ErrIdempotencyKeyInvalid):
		return KeyStatus(ReasonInvalidIdempotencyKey), true
	case errors.Is(err, ports.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "already exists"), true
	default:
		return nil, false
	}
}

// KeyStatus is the edge's refusal of a command whose metadata carries no key or
// a key outside ports.IdempotencyKeyPattern (IDM-01, IDM-02).
func KeyStatus(reason string) error {
	message := "idempotency key required"
	if reason == ReasonInvalidIdempotencyKey {
		message = "idempotency key outside " + ports.IdempotencyKeyPattern
	}
	return withReason(codes.InvalidArgument, message, reason)
}

// ReasonOf reads the reason a status carries in its ErrorInfo, or "" when it
// carries none.
func ReasonOf(err error) string {
	st, ok := status.FromError(err)
	if !ok || st == nil {
		return ""
	}
	for _, detail := range st.Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok && info.GetDomain() == errorInfoDomain {
			return info.GetReason()
		}
	}
	return ""
}

func withReason(code codes.Code, message, reason string) error {
	st := status.New(code, message)
	detailed, err := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: errorInfoDomain})
	if err != nil {
		return st.Err()
	}
	return detailed.Err()
}
