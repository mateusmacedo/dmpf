package rpc

import (
	"context"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	ordersapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application/example/orders"
	servicev1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/orders/service/v1"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain/example/orders"
)

var descriptor = servicev1.File_company_orders_service_v1_orders_service_proto.Services().ByName("OrdersService")

// ServiceName is the full name the generated descriptor gives the service.
var ServiceName = string(descriptor.FullName())

var orderIDFormat = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// FullMethod is the wire name of a method of the service: /<service>/<method>.
func FullMethod(name string) string {
	return "/" + ServiceName + "/" + name
}

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
		unary(method("AddItem"), OrdersServer.AddItem),
		unary(method("PlaceOrder"), OrdersServer.PlaceOrder),
		unary(method("FindOrder"), OrdersServer.FindOrder),
	},
	Metadata: descriptor.ParentFile().Path(),
}

func method(name protoreflect.Name) protoreflect.MethodDescriptor {
	md := descriptor.Methods().ByName(name)
	if md == nil {
		panic("rpc: OrdersService declares no method " + string(name))
	}
	return md
}

func unary[Req any, PReq interface {
	*Req
	proto.Message
}, Resp proto.Message](md protoreflect.MethodDescriptor, call func(OrdersServer, context.Context, PReq) (Resp, error)) grpc.MethodDesc {
	name := string(md.Name())
	fullMethod := FullMethod(name)
	invoke := func(server OrdersServer, ctx context.Context, req PReq) (any, error) {
		resp, err := call(server, ctx, req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	return grpc.MethodDesc{
		MethodName: name,
		Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			in := PReq(new(Req))
			if err := dec(in); err != nil {
				return nil, err
			}
			server := srv.(OrdersServer)
			if interceptor == nil {
				return invoke(server, ctx, in)
			}
			info := &grpc.UnaryServerInfo{Server: srv, FullMethod: fullMethod}
			return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
				return invoke(server, ctx, req.(PReq))
			})
		},
	}
}

// Server realizes OrdersServer over the orders use cases.
type Server struct {
	Service ordersapp.Service
}

func (s Server) AddItem(ctx context.Context, req *servicev1.AddItemRequest) (*servicev1.AddItemResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	out, err := s.Service.AddItem(ctx, ordersapp.AddItem{Order: id, SKU: orders.SKU(req.GetSku()), Quantity: int(req.GetQuantity())})
	if err != nil {
		return nil, statusOf(err)
	}
	if rejection, refused := out.Rejection(); refused {
		return &servicev1.AddItemResponse{Result: &servicev1.AddItemResponse_Rejection{Rejection: rejectionOf(rejection)}}, nil
	}
	accepted := out.Response()
	return &servicev1.AddItemResponse{Result: &servicev1.AddItemResponse_Accepted{
		Accepted: &servicev1.ItemAccepted{OrderId: string(accepted.Order), ItemCount: int32(accepted.Items)},
	}}, nil
}

func (s Server) PlaceOrder(ctx context.Context, req *servicev1.PlaceOrderRequest) (*servicev1.PlaceOrderResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	out, err := s.Service.PlaceOrder(ctx, ordersapp.PlaceOrder{Order: id})
	if err != nil {
		return nil, statusOf(err)
	}
	if rejection, refused := out.Rejection(); refused {
		return &servicev1.PlaceOrderResponse{Result: &servicev1.PlaceOrderResponse_Rejection{Rejection: rejectionOf(rejection)}}, nil
	}
	return &servicev1.PlaceOrderResponse{Result: &servicev1.PlaceOrderResponse_Placed{
		Placed: &servicev1.Placed{OrderId: string(out.Response().Order)},
	}}, nil
}

func (s Server) FindOrder(ctx context.Context, req *servicev1.FindOrderRequest) (*servicev1.FindOrderResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	snapshot, err := s.Service.FindOrder(ctx, id)
	if err != nil {
		return nil, statusOf(err)
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

func orderID(raw string) (orders.OrderID, error) {
	if !orderIDFormat.MatchString(raw) {
		return "", status.Error(codes.InvalidArgument, "order_id must have 1 to 128 characters of [A-Za-z0-9._:-]")
	}
	return orders.OrderID(raw), nil
}

func orderStatus(s orders.Status) servicev1.OrderStatus {
	switch s {
	case orders.Open:
		return servicev1.OrderStatus_ORDER_STATUS_OPEN
	case orders.Placed:
		return servicev1.OrderStatus_ORDER_STATUS_PLACED
	default:
		return servicev1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func rejectionOf(rejection *dmpfdomain.Rejection) *servicev1.Rejection {
	return &servicev1.Rejection{Code: string(rejection.Code()), Message: rejection.Message()}
}
