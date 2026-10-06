package rpc_test

import (
	"slices"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app/rpc"
	servicev1 "github.com/mateusmacedo/dmpf/apps/backend/reservations/contract/gen/go/company/reservations/service/v1"
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
		query := method == "FindReservation"
		if query == slices.Contains(commands, method) {
			t.Errorf("method %s: query %v, command %v; every method is exactly one of the two", method, query, !query)
		}
	}
}

func TestACommandWithoutAKeyIsRefusedBeforeTheHandler(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.ReserveResponse
	err := h.invoke(withoutKey(t), "Reserve", &servicev1.ReserveRequest{OrderId: "o-1", ItemCount: 2}, &resp)

	if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
		t.Fatalf("Reserve() = %v (reason %q), want InvalidArgument with %s", err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonMissingIdempotencyKey)
	}
	if got := h.store.WithinCalls(); got != 0 {
		t.Fatalf("WithinCalls() = %d, want 0", got)
	}
}

func TestACommandWithAMalformedKeyIsRefused(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.CancelResponse
	err := h.invoke(withKey(t, "not a key"), "Cancel", &servicev1.CancelRequest{OrderId: "o-1"}, &resp)

	if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonInvalidIdempotencyKey {
		t.Fatalf("Cancel() = %v (reason %q), want InvalidArgument with %s", err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonInvalidIdempotencyKey)
	}
}

func TestARepeatedCommandAnswersTheSameResponseMarkedAsReplay(t *testing.T) {
	h := newHarness(t, unlimited)
	req := &servicev1.ReserveRequest{OrderId: "o-1", ItemCount: 2}

	var first, again servicev1.ReserveResponse
	var firstHeader, againHeader metadata.MD
	if err := h.invoke(withKey(t, "k-replay"), "Reserve", req, &first, grpc.Header(&firstHeader)); err != nil {
		t.Fatalf("first Reserve() = %v", err)
	}
	if err := h.invoke(withKey(t, "k-replay"), "Reserve", req, &again, grpc.Header(&againHeader)); err != nil {
		t.Fatalf("repeated Reserve() = %v, want the stored response", err)
	}

	if again.GetReserved().GetItemCount() != first.GetReserved().GetItemCount() || again.GetReserved().GetItemCount() != 2 {
		t.Fatalf("replay = %v, want %v", &again, &first)
	}
	if n := len(h.store.Entries()); n != 1 {
		t.Fatalf("Entries() = %d, want 1: the replay produced a second event", n)
	}
	if got := firstHeader.Get(kernelgrpc.ReplayedHeader); len(got) != 0 {
		t.Fatalf("first response header %s = %v, want none", kernelgrpc.ReplayedHeader, got)
	}
	if got := againHeader.Get(kernelgrpc.ReplayedHeader); !slices.Equal(got, []string{"true"}) {
		t.Fatalf("replay header %s = %v, want [true]", kernelgrpc.ReplayedHeader, got)
	}
}

func TestTheSameKeyWithAnotherRequestIsReused(t *testing.T) {
	h := newHarness(t, unlimited)
	var resp servicev1.ReserveResponse
	if err := h.invoke(withKey(t, "k-reused"), "Reserve", &servicev1.ReserveRequest{OrderId: "o-1", ItemCount: 2}, &resp); err != nil {
		t.Fatalf("first Reserve() = %v", err)
	}

	err := h.invoke(withKey(t, "k-reused"), "Reserve", &servicev1.ReserveRequest{OrderId: "o-1", ItemCount: 3}, &resp)

	if status.Code(err) != codes.FailedPrecondition || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonReusedIdempotencyKey {
		t.Fatalf("Reserve(other count) = %v (reason %q), want FailedPrecondition with %s", err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonReusedIdempotencyKey)
	}
}
