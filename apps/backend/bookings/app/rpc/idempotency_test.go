package rpc_test

import (
	"slices"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app/rpc"
	servicev1 "github.com/mateusmacedo/dmpf/apps/backend/bookings/contract/gen/go/company/bookings/service/v1"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
)

func TestEveryMethodThatIsNotAQueryIsACommand(t *testing.T) {
	commands := rpc.Commands()
	for _, command := range commands {
		if !slices.Contains(rpc.Methods(), command) {
			t.Errorf("command %s is not a method of %s", command, rpc.ServiceName)
		}
	}
	for _, method := range rpc.Methods() {
		query := method == "FindBooking" || method == "FindBookingsByResource"
		if query == slices.Contains(commands, method) {
			t.Errorf("method %s: query %v, command %v; every method is exactly one of the two", method, query, !query)
		}
	}
}

func TestACommandWithoutAKeyIsRefusedBeforeTheHandler(t *testing.T) {
	h := newHarness(t, nil)

	var resp servicev1.RegisterResourceResponse
	err := h.invoke(withoutKey(t), "RegisterResource", &servicev1.RegisterResourceRequest{ResourceId: "room-1"}, &resp)

	if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
		t.Fatalf("RegisterResource() = %v (reason %q), want InvalidArgument with %s", err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonMissingIdempotencyKey)
	}
}

func TestACommandWithAMalformedKeyIsRefused(t *testing.T) {
	h := newHarness(t, nil)

	var resp servicev1.RegisterResourceResponse
	err := h.invoke(withKey(t, "not a key"), "RegisterResource", &servicev1.RegisterResourceRequest{ResourceId: "room-1"}, &resp)

	if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonInvalidIdempotencyKey {
		t.Fatalf("RegisterResource() = %v (reason %q), want InvalidArgument with %s", err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonInvalidIdempotencyKey)
	}
}

func TestARepeatedCommandAnswersTheSameResponseMarkedAsReplay(t *testing.T) {
	h := newHarness(t, nil)
	req := &servicev1.ReserveBookingRequest{BookingId: "b-1", ResourceId: "room-1", Quantity: 2}

	var first, again servicev1.ReserveBookingResponse
	var firstHeader, againHeader metadata.MD
	if err := h.invoke(withKey(t, "k-replay"), "ReserveBooking", req, &first, grpc.Header(&firstHeader)); err != nil {
		t.Fatalf("first ReserveBooking() = %v", err)
	}
	if err := h.invoke(withKey(t, "k-replay"), "ReserveBooking", req, &again, grpc.Header(&againHeader)); err != nil {
		t.Fatalf("repeated ReserveBooking() = %v, want the stored response", err)
	}

	if again.GetReserved().GetBookingId() != first.GetReserved().GetBookingId() {
		t.Fatalf("replay = %v, want %v", &again, &first)
	}
	if got := firstHeader.Get(kernelgrpc.ReplayedHeader); len(got) != 0 {
		t.Fatalf("first response header %s = %v, want none", kernelgrpc.ReplayedHeader, got)
	}
	if got := againHeader.Get(kernelgrpc.ReplayedHeader); !slices.Equal(got, []string{"true"}) {
		t.Fatalf("replay header %s = %v, want [true]", kernelgrpc.ReplayedHeader, got)
	}
}

func TestTheSameKeyWithAnotherRequestIsReused(t *testing.T) {
	h := newHarness(t, nil)
	var resp servicev1.ReserveBookingResponse
	if err := h.invoke(withKey(t, "k-reused"), "ReserveBooking", &servicev1.ReserveBookingRequest{BookingId: "b-1", ResourceId: "room-1", Quantity: 2}, &resp); err != nil {
		t.Fatalf("first ReserveBooking() = %v", err)
	}

	err := h.invoke(withKey(t, "k-reused"), "ReserveBooking", &servicev1.ReserveBookingRequest{BookingId: "b-1", ResourceId: "room-1", Quantity: 3}, &resp)

	if status.Code(err) != codes.FailedPrecondition || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonReusedIdempotencyKey {
		t.Fatalf("ReserveBooking(other quantity) = %v (reason %q), want FailedPrecondition with %s", err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonReusedIdempotencyKey)
	}
}
