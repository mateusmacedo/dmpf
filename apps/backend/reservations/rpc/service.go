package rpc

import (
	"context"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	servicev1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
)

var descriptor = servicev1.File_company_reservations_service_v1_reservations_service_proto.Services().ByName("ReservationsService")

// ServiceName is the full name the generated descriptor gives the service.
var ServiceName = string(descriptor.FullName())

var orderIDFormat = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// FullMethod is the wire name of a method of the service: /<service>/<method>.
func FullMethod(name string) string {
	return "/" + ServiceName + "/" + name
}

// ReservationsServer is the handler type ServiceDesc registers.
type ReservationsServer interface {
	Reserve(context.Context, *servicev1.ReserveRequest) (*servicev1.ReserveResponse, error)
	Cancel(context.Context, *servicev1.CancelRequest) (*servicev1.CancelResponse, error)
	FindReservation(context.Context, *servicev1.FindReservationRequest) (*servicev1.FindReservationResponse, error)
}

// ServiceDesc binds ReservationsService in the app block: protoc-gen-go-grpc
// would put io.network into the contract block, which admits only wire.codec.
var ServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*ReservationsServer)(nil),
	Methods: []grpc.MethodDesc{
		unary(method("Reserve"), ReservationsServer.Reserve),
		unary(method("Cancel"), ReservationsServer.Cancel),
		unary(method("FindReservation"), ReservationsServer.FindReservation),
	},
	Metadata: descriptor.ParentFile().Path(),
}

func method(name protoreflect.Name) protoreflect.MethodDescriptor {
	md := descriptor.Methods().ByName(name)
	if md == nil {
		panic("rpc: ReservationsService declares no method " + string(name))
	}
	return md
}

func unary[Req any, PReq interface {
	*Req
	proto.Message
}, Resp proto.Message](md protoreflect.MethodDescriptor, call func(ReservationsServer, context.Context, PReq) (Resp, error)) grpc.MethodDesc {
	name := string(md.Name())
	fullMethod := FullMethod(name)
	invoke := func(server ReservationsServer, ctx context.Context, req PReq) (any, error) {
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
			server := srv.(ReservationsServer)
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

// Server realizes ReservationsServer over the reservations use cases.
type Server struct {
	Service application.Service
}

func (s Server) Reserve(ctx context.Context, req *servicev1.ReserveRequest) (*servicev1.ReserveResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	out, err := s.Service.Reserve(ctx, application.Reserve{Order: id, Items: int(req.GetItemCount())})
	if err != nil {
		return nil, statusOf(err)
	}
	if rejection, refused := out.Rejection(); refused {
		return &servicev1.ReserveResponse{Result: &servicev1.ReserveResponse_Rejection{Rejection: rejectionOf(rejection)}}, nil
	}
	reserved := out.Response()
	return &servicev1.ReserveResponse{Result: &servicev1.ReserveResponse_Reserved{
		Reserved: &servicev1.Reserved{OrderId: string(reserved.Order), ItemCount: int32(reserved.Items)},
	}}, nil
}

func (s Server) Cancel(ctx context.Context, req *servicev1.CancelRequest) (*servicev1.CancelResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	out, err := s.Service.Cancel(ctx, application.Cancel{Order: id})
	if err != nil {
		return nil, statusOf(err)
	}
	if rejection, refused := out.Rejection(); refused {
		return &servicev1.CancelResponse{Result: &servicev1.CancelResponse_Rejection{Rejection: rejectionOf(rejection)}}, nil
	}
	return &servicev1.CancelResponse{Result: &servicev1.CancelResponse_Canceled{
		Canceled: &servicev1.Canceled{OrderId: string(out.Response().Order)},
	}}, nil
}

func (s Server) FindReservation(ctx context.Context, req *servicev1.FindReservationRequest) (*servicev1.FindReservationResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	snapshot, err := s.Service.FindReservation(ctx, id)
	if err != nil {
		return nil, statusOf(err)
	}
	return &servicev1.FindReservationResponse{Reservation: &servicev1.Reservation{
		OrderId:   string(snapshot.Order),
		Status:    reservationStatus(snapshot.Status),
		ItemCount: int32(snapshot.Items),
	}}, nil
}

func orderID(raw string) (domain.OrderID, error) {
	if !orderIDFormat.MatchString(raw) {
		return "", status.Error(codes.InvalidArgument, "order_id must have 1 to 128 characters of [A-Za-z0-9._:-]")
	}
	return domain.OrderID(raw), nil
}

func reservationStatus(s domain.Status) servicev1.ReservationStatus {
	switch s {
	case domain.Confirmed:
		return servicev1.ReservationStatus_RESERVATION_STATUS_CONFIRMED
	case domain.Canceled:
		return servicev1.ReservationStatus_RESERVATION_STATUS_CANCELED
	default:
		return servicev1.ReservationStatus_RESERVATION_STATUS_UNSPECIFIED
	}
}

func rejectionOf(rejection *kernel.Rejection) *servicev1.Rejection {
	return &servicev1.Rejection{Code: string(rejection.Code()), Message: rejection.Message()}
}
