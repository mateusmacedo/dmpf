package rpc_test

import (
	"reflect"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	servicev1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app/rpc"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

func TestServiceDescCoversEveryMethodOfTheDescriptor(t *testing.T) {
	service := servicev1.File_company_reservations_service_v1_reservations_service_proto.Services().ByName("ReservationsService")

	if rpc.ServiceDesc.ServiceName != string(service.FullName()) {
		t.Fatalf("ServiceName = %q, want %q", rpc.ServiceDesc.ServiceName, service.FullName())
	}
	declared := map[string]bool{}
	for _, m := range rpc.ServiceDesc.Methods {
		declared[m.MethodName] = true
	}
	methods := service.Methods()
	if len(rpc.ServiceDesc.Methods) != methods.Len() || len(rpc.ServiceDesc.Streams) != 0 {
		t.Fatalf("ServiceDesc has %d unary and %d stream methods, want the %d unary of the descriptor",
			len(rpc.ServiceDesc.Methods), len(rpc.ServiceDesc.Streams), methods.Len())
	}
	for i := range methods.Len() {
		if name := string(methods.Get(i).Name()); !declared[name] {
			t.Fatalf("descriptor method %q has no handler", name)
		}
	}
}

func TestTheServerImplementsTheHandlerType(t *testing.T) {
	handlerType := reflect.TypeOf(rpc.ServiceDesc.HandlerType).Elem()
	if handlerType.Kind() != reflect.Interface || !reflect.TypeOf(rpc.Server{}).Implements(handlerType) {
		t.Fatalf("rpc.Server does not implement %v", handlerType)
	}
}

func TestReserveAcceptedReturnsTheItemCount(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.ReserveResponse
	err := h.invoke(withTenant(t), "Reserve", &servicev1.ReserveRequest{OrderId: "o-1", ItemCount: 2}, &resp)

	if err != nil {
		t.Fatalf("Reserve() = %v, want nil", err)
	}
	if got := resp.GetReserved(); got.GetOrderId() != "o-1" || got.GetItemCount() != 2 {
		t.Fatalf("reserved = %v, want o-1 with 2 items", got)
	}
}

func TestCancelAcceptedReturnsTheOrder(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.CancelResponse
	err := h.invoke(withTenant(t), "Cancel", &servicev1.CancelRequest{OrderId: "o-2"}, &resp)

	if err != nil {
		t.Fatalf("Cancel() = %v, want nil", err)
	}
	if got := resp.GetCanceled(); got.GetOrderId() != "o-2" {
		t.Fatalf("canceled = %v, want o-2", got)
	}
}

func TestCancelAfterReserveTravelsAsARejection(t *testing.T) {
	h := newHarness(t, unlimited)
	var reserved servicev1.ReserveResponse
	if err := h.invoke(withTenant(t), "Reserve", &servicev1.ReserveRequest{OrderId: "o-1", ItemCount: 1}, &reserved); err != nil {
		t.Fatalf("setup Reserve() = %v", err)
	}

	var resp servicev1.CancelResponse
	err := h.invoke(withTenant(t), "Cancel", &servicev1.CancelRequest{OrderId: "o-1"}, &resp)

	if err != nil {
		t.Fatalf("Cancel() = %v, want nil — a domain refusal is not a gRPC error", err)
	}
	if got := resp.GetRejection(); got.GetCode() != string(domain.CodeAlreadyReserved) || got.GetMessage() == "" {
		t.Fatalf("rejection = %v, want %q — the first decision won", got, domain.CodeAlreadyReserved)
	}
}

func TestFindReservationReturnsTheStatus(t *testing.T) {
	h := newHarness(t, unlimited)
	var reserved servicev1.ReserveResponse
	if err := h.invoke(withTenant(t), "Reserve", &servicev1.ReserveRequest{OrderId: "o-1", ItemCount: 2}, &reserved); err != nil {
		t.Fatalf("setup Reserve() = %v", err)
	}
	var canceled servicev1.CancelResponse
	if err := h.invoke(withTenant(t), "Cancel", &servicev1.CancelRequest{OrderId: "o-2"}, &canceled); err != nil {
		t.Fatalf("setup Cancel() = %v", err)
	}

	var confirmed, cancel servicev1.FindReservationResponse
	if err := h.invoke(withTenant(t), "FindReservation", &servicev1.FindReservationRequest{OrderId: "o-1"}, &confirmed); err != nil {
		t.Fatalf("FindReservation(o-1) = %v", err)
	}
	if err := h.invoke(withTenant(t), "FindReservation", &servicev1.FindReservationRequest{OrderId: "o-2"}, &cancel); err != nil {
		t.Fatalf("FindReservation(o-2) = %v", err)
	}

	if got := confirmed.GetReservation(); got.GetStatus() != servicev1.ReservationStatus_RESERVATION_STATUS_CONFIRMED || got.GetItemCount() != 2 {
		t.Fatalf("o-1 = %v, want confirmed with 2 items", got)
	}
	if got := cancel.GetReservation(); got.GetStatus() != servicev1.ReservationStatus_RESERVATION_STATUS_CANCELED || got.GetOrderId() != "o-2" {
		t.Fatalf("o-2 = %v, want canceled", got)
	}
}

func TestAnAbsentReservationIsNotFound(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.FindReservationResponse
	err := h.invoke(withTenant(t), "FindReservation", &servicev1.FindReservationRequest{OrderId: "o-404"}, &resp)

	if status.Code(err) != codes.NotFound {
		t.Fatalf("FindReservation() = %v, want NotFound", err)
	}
	if msg := status.Convert(err).Message(); msg == "" || len(msg) > 64 {
		t.Fatalf("status message = %q, want a short generic message without internal detail", msg)
	}
}

func TestAnInvalidOrderIDIsInvalidArgumentWithoutTheUseCase(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.ReserveResponse
	err := h.invoke(withDeadline(t), "Reserve", &servicev1.ReserveRequest{OrderId: "", ItemCount: 1}, &resp)

	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Reserve() = %v, want InvalidArgument", err)
	}
	if got := h.store.WithinCalls(); got != 0 {
		t.Fatalf("WithinCalls() = %d, want 0", got)
	}
}
