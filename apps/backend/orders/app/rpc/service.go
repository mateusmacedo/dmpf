package rpc

import (
	"context"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	servicev1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/service/v1"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// WHY: the absence of what the interceptor mounted is a wiring defect of this
// server, never a fault of the caller, so it answers Internal rather than
// leaving the handler to invent a context (CTX-03).
func executionOf(ctx context.Context) (ports.ExecutionContext, error) {
	execution, ok := ports.ExecutionContextFrom(ctx)
	if !ok {
		return ports.ExecutionContext{}, status.Error(codes.Internal, "the execution context was not assembled")
	}
	return execution, nil
}

var descriptor = servicev1.File_company_orders_service_v1_orders_service_proto.Services().ByName("OrdersService")

// ServiceName is the full name the generated descriptor gives the service.
var ServiceName = string(descriptor.FullName())

var orderIDFormat = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// Methods names every method of the service: admission declares a limit for
// each one (RES-16).
func Methods() []string { return kernelgrpc.MethodNames(descriptor) }

// Commands lists the methods that require an idempotency key (IDM-01); the
// queries neither require nor read one.
func Commands() []string {
	return []string{"AddItem", "PlaceOrder"}
}

// FullMethod is the wire name of a method of the service: /<service>/<method>.
func FullMethod(name string) string { return kernelgrpc.FullMethod(ServiceName, name) }

// OrdersServer is the handler type ServiceDesc registers.
type OrdersServer interface {
	AddItem(context.Context, *servicev1.AddItemRequest) (*servicev1.AddItemResponse, error)
	PlaceOrder(context.Context, *servicev1.PlaceOrderRequest) (*servicev1.PlaceOrderResponse, error)
	FindOrder(context.Context, *servicev1.FindOrderRequest) (*servicev1.FindOrderResponse, error)
}

// ServiceDesc binds OrdersService in the app block: protoc-gen-go-grpc would put
// io.network into the contract block, which admits only wire.codec (ADR-015).
var ServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*OrdersServer)(nil),
	Methods: []grpc.MethodDesc{
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "AddItem"), OrdersServer.AddItem),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "PlaceOrder"), OrdersServer.PlaceOrder),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindOrder"), OrdersServer.FindOrder),
	},
	Metadata: descriptor.ParentFile().Path(),
}

// Server realizes OrdersServer over the orders use cases.
type Server struct {
	Service application.Service
}

func command[R, Resp any](ctx context.Context, run func(context.Context) (usecase.Outcome[R], error), refused func(*servicev1.Rejection) Resp, accepted func(R) Resp) (Resp, error) {
	var none Resp
	if _, err := executionOf(ctx); err != nil {
		return none, err
	}
	out, err := run(ctx)
	if err != nil {
		return none, kernelgrpc.StatusOf(err)
	}
	if rejection, declined := out.Rejection(); declined {
		return refused(rejectionOf(rejection)), nil
	}
	return accepted(out.Response()), nil
}

func (s Server) AddItem(ctx context.Context, req *servicev1.AddItemRequest) (*servicev1.AddItemResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	return command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ItemAccepted], error) {
			return s.Service.AddItem(ctx, application.AddItem{Order: id, SKU: domain.SKU(req.GetSku()), Quantity: int(req.GetQuantity())})
		},
		addItemRefused,
		itemAcceptedOf)
}

func (s Server) PlaceOrder(ctx context.Context, req *servicev1.PlaceOrderRequest) (*servicev1.PlaceOrderResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	return command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.PlacedResponse], error) {
			return s.Service.PlaceOrder(ctx, application.PlaceOrder{Order: id})
		},
		placeOrderRefused,
		orderPlacedOf)
}

func addItemRefused(r *servicev1.Rejection) *servicev1.AddItemResponse {
	return &servicev1.AddItemResponse{Result: &servicev1.AddItemResponse_Rejection{Rejection: r}}
}

func itemAcceptedOf(a domain.ItemAccepted) *servicev1.AddItemResponse {
	return &servicev1.AddItemResponse{Result: &servicev1.AddItemResponse_Accepted{
		Accepted: &servicev1.ItemAccepted{OrderId: string(a.Order), ItemCount: int32(a.Items)},
	}}
}

func placeOrderRefused(r *servicev1.Rejection) *servicev1.PlaceOrderResponse {
	return &servicev1.PlaceOrderResponse{Result: &servicev1.PlaceOrderResponse_Rejection{Rejection: r}}
}

func orderPlacedOf(p domain.PlacedResponse) *servicev1.PlaceOrderResponse {
	return &servicev1.PlaceOrderResponse{Result: &servicev1.PlaceOrderResponse_Placed{Placed: &servicev1.Placed{OrderId: string(p.Order)}}}
}

func (s Server) FindOrder(ctx context.Context, req *servicev1.FindOrderRequest) (*servicev1.FindOrderResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	_, err = executionOf(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.Service.FindOrder(ctx, id)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	items := make([]*servicev1.Item, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		items = append(items, &servicev1.Item{Sku: string(item.SKU), Quantity: int32(item.Quantity)})
	}
	return &servicev1.FindOrderResponse{Order: &servicev1.Order{
		OrderId:   string(snapshot.ID),
		Status:    orderStatus(snapshot.Status),
		ItemLimit: int32(snapshot.ItemLimit),
		Items:     items,
	}}, nil
}

func orderID(raw string) (domain.OrderID, error) {
	if !orderIDFormat.MatchString(raw) {
		return "", status.Error(codes.InvalidArgument, "order_id must have 1 to 128 characters of [A-Za-z0-9._:-]")
	}
	return domain.OrderID(raw), nil
}

func orderStatus(s domain.Status) servicev1.OrderStatus {
	switch s {
	case domain.Open:
		return servicev1.OrderStatus_ORDER_STATUS_OPEN
	case domain.Placed:
		return servicev1.OrderStatus_ORDER_STATUS_PLACED
	default:
		return servicev1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func rejectionOf(rejection *kernel.Rejection) *servicev1.Rejection {
	return &servicev1.Rejection{Code: string(rejection.Code()), Message: rejection.Message()}
}
