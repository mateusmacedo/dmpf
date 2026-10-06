package rpc

import (
	"context"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	servicev1 "github.com/mateusmacedo/dmpf/apps/backend/reservations/contract/gen/go/company/reservations/service/v1"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
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

var descriptor = servicev1.File_company_reservations_service_v1_reservations_service_proto.Services().ByName("ReservationsService")

// ServiceName is the full name the generated descriptor gives the service.
var ServiceName = string(descriptor.FullName())

var orderIDFormat = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// Methods names every method of the service: admission declares a limit for
// each one (RES-16).
func Methods() []string { return kernelgrpc.MethodNames(descriptor) }

// Commands lists the methods that require an idempotency key (IDM-01); the
// queries neither require nor read one.
func Commands() []string {
	return []string{"Reserve", "Cancel"}
}

// FullMethod is the wire name of a method of the service: /<service>/<method>.
func FullMethod(name string) string { return kernelgrpc.FullMethod(ServiceName, name) }

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
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "Reserve"), ReservationsServer.Reserve),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "Cancel"), ReservationsServer.Cancel),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindReservation"), ReservationsServer.FindReservation),
	},
	Metadata: descriptor.ParentFile().Path(),
}

// Server realizes ReservationsServer over the reservations use cases.
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

func (s Server) Reserve(ctx context.Context, req *servicev1.ReserveRequest) (*servicev1.ReserveResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	return command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ReservedResponse], error) {
			return s.Service.Reserve(ctx, application.Reserve{Order: id, Items: int(req.GetItemCount())})
		},
		reserveRefused,
		reservedOf)
}

func (s Server) Cancel(ctx context.Context, req *servicev1.CancelRequest) (*servicev1.CancelResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	return command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.CancelledResponse], error) {
			return s.Service.Cancel(ctx, application.Cancel{Order: id})
		},
		cancelRefused,
		canceledOf)
}

func reserveRefused(r *servicev1.Rejection) *servicev1.ReserveResponse {
	return &servicev1.ReserveResponse{Result: &servicev1.ReserveResponse_Rejection{Rejection: r}}
}

func reservedOf(reserved domain.ReservedResponse) *servicev1.ReserveResponse {
	return &servicev1.ReserveResponse{Result: &servicev1.ReserveResponse_Reserved{
		Reserved: &servicev1.Reserved{OrderId: string(reserved.Order), ItemCount: int32(reserved.Items)},
	}}
}

func cancelRefused(r *servicev1.Rejection) *servicev1.CancelResponse {
	return &servicev1.CancelResponse{Result: &servicev1.CancelResponse_Rejection{Rejection: r}}
}

func canceledOf(c domain.CancelledResponse) *servicev1.CancelResponse {
	return &servicev1.CancelResponse{Result: &servicev1.CancelResponse_Canceled{Canceled: &servicev1.Canceled{OrderId: string(c.Order)}}}
}

func (s Server) FindReservation(ctx context.Context, req *servicev1.FindReservationRequest) (*servicev1.FindReservationResponse, error) {
	id, err := orderID(req.GetOrderId())
	if err != nil {
		return nil, err
	}
	_, err = executionOf(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.Service.FindReservation(ctx, id)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
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
	case domain.Cancelled:
		return servicev1.ReservationStatus_RESERVATION_STATUS_CANCELED
	default:
		return servicev1.ReservationStatus_RESERVATION_STATUS_UNSPECIFIED
	}
}

func rejectionOf(rejection *kernel.Rejection) *servicev1.Rejection {
	return &servicev1.Rejection{Code: string(rejection.Code()), Message: rejection.Message()}
}
