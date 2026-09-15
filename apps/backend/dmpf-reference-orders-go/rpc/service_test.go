package rpc_test

import (
	"reflect"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference-orders-go/rpc"
	servicev1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/orders/service/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/orders"
)

func TestServiceDescCoversEveryMethodOfTheDescriptor(t *testing.T) {
	service := servicev1.File_company_orders_service_v1_orders_service_proto.Services().ByName("OrdersService")

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

func TestAddItemAcceptedReturnsTheItemCount(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.AddItemResponse
	err := h.invoke(withDeadline(t), "AddItem", &servicev1.AddItemRequest{OrderId: "o-1", Sku: "A", Quantity: 1}, &resp)

	if err != nil {
		t.Fatalf("AddItem() = %v, want nil", err)
	}
	if got := resp.GetAccepted(); got.GetOrderId() != "o-1" || got.GetItemCount() != 1 {
		t.Fatalf("accepted = %v, want order o-1 with 1 item", got)
	}
}

func TestAddItemRejectionTravelsInTheResponse(t *testing.T) {
	h := newHarness(t, unlimited)
	var first servicev1.AddItemResponse
	if err := h.invoke(withDeadline(t), "AddItem", &servicev1.AddItemRequest{OrderId: "o-1", Sku: "A", Quantity: 1}, &first); err != nil {
		t.Fatalf("setup AddItem() = %v", err)
	}

	var resp servicev1.AddItemResponse
	err := h.invoke(withDeadline(t), "AddItem", &servicev1.AddItemRequest{OrderId: "o-1", Sku: "B", Quantity: 1}, &resp)

	if err != nil {
		t.Fatalf("AddItem() = %v, want nil — a domain refusal is not a gRPC error", err)
	}
	if got := resp.GetRejection(); got.GetCode() != string(orders.CodeItemLimitExceeded) || got.GetMessage() == "" {
		t.Fatalf("rejection = %v, want %q with a message", got, orders.CodeItemLimitExceeded)
	}
}

func TestFindOrderReturnsTheOrder(t *testing.T) {
	h := newHarness(t, unlimited)
	var added servicev1.AddItemResponse
	if err := h.invoke(withDeadline(t), "AddItem", &servicev1.AddItemRequest{OrderId: "o-1", Sku: "A", Quantity: 2}, &added); err != nil {
		t.Fatalf("setup AddItem() = %v", err)
	}

	var resp servicev1.FindOrderResponse
	err := h.invoke(withDeadline(t), "FindOrder", &servicev1.FindOrderRequest{OrderId: "o-1"}, &resp)

	if err != nil {
		t.Fatalf("FindOrder() = %v, want nil", err)
	}
	order := resp.GetOrder()
	if order.GetOrderId() != "o-1" || order.GetStatus() != servicev1.OrderStatus_ORDER_STATUS_OPEN || order.GetItemLimit() != 1 {
		t.Fatalf("order = %v, want o-1 open with item limit 1", order)
	}
	if items := order.GetItems(); len(items) != 1 || items[0].GetSku() != "A" || items[0].GetQuantity() != 2 {
		t.Fatalf("items = %v, want one A x2", items)
	}
}

func TestAbsentOrdersAreNotFound(t *testing.T) {
	h := newHarness(t, unlimited)

	var find servicev1.FindOrderResponse
	findErr := h.invoke(withDeadline(t), "FindOrder", &servicev1.FindOrderRequest{OrderId: "o-404"}, &find)
	var place servicev1.PlaceOrderResponse
	placeErr := h.invoke(withDeadline(t), "PlaceOrder", &servicev1.PlaceOrderRequest{OrderId: "o-404"}, &place)

	if status.Code(findErr) != codes.NotFound || status.Code(placeErr) != codes.NotFound {
		t.Fatalf("FindOrder = %v, PlaceOrder = %v; want NotFound for both", findErr, placeErr)
	}
	if msg := status.Convert(placeErr).Message(); msg == "" || len(msg) > 64 {
		t.Fatalf("status message = %q, want a short generic message without internal detail", msg)
	}
}

func TestAnInvalidOrderIDIsInvalidArgumentWithoutTheUseCase(t *testing.T) {
	h := newHarness(t, unlimited)

	var resp servicev1.AddItemResponse
	err := h.invoke(withDeadline(t), "AddItem", &servicev1.AddItemRequest{OrderId: "", Sku: "A", Quantity: 1}, &resp)

	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("AddItem() = %v, want InvalidArgument", err)
	}
	if got := h.store.WithinCalls(); got != 0 {
		t.Fatalf("WithinCalls() = %d, want 0", got)
	}
}
