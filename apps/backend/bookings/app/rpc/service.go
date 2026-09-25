package rpc

import (
	"context"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	servicev1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/bookings/service/v1"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	maxResourceID = 128
	minQuantity   = 1
	maxQuantity   = 100
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

var descriptor = servicev1.File_company_bookings_service_v1_bookings_service_proto.Services().ByName("BookingsService")

// ServiceName is the full name the generated descriptor gives the service.
var ServiceName = string(descriptor.FullName())

var bookingIDFormat = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// Methods names every method of the service: admission declares a limit for
// each one (RES-16).
func Methods() []string {
	methods := descriptor.Methods()
	names := make([]string, 0, methods.Len())
	for i := range methods.Len() {
		names = append(names, string(methods.Get(i).Name()))
	}
	return names
}

// FullMethod is the wire name of a method of the service: /<service>/<method>.
func FullMethod(name string) string {
	return "/" + ServiceName + "/" + name
}

// BookingsServer is the handler type ServiceDesc registers.
type BookingsServer interface {
	ReserveBooking(context.Context, *servicev1.ReserveBookingRequest) (*servicev1.ReserveBookingResponse, error)
	CancelBooking(context.Context, *servicev1.CancelBookingRequest) (*servicev1.CancelBookingResponse, error)
	RegisterResource(context.Context, *servicev1.RegisterResourceRequest) (*servicev1.RegisterResourceResponse, error)
	FindBooking(context.Context, *servicev1.FindBookingRequest) (*servicev1.FindBookingResponse, error)
	FindBookingsByResource(context.Context, *servicev1.FindBookingsByResourceRequest) (*servicev1.FindBookingsByResourceResponse, error)
}

// ServiceDesc binds BookingsService in the app block: protoc-gen-go-grpc would
// put io.network into the contract block, which admits only wire.codec (ADR-015).
var ServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*BookingsServer)(nil),
	Methods: []grpc.MethodDesc{
		unary(method("ReserveBooking"), BookingsServer.ReserveBooking),
		unary(method("CancelBooking"), BookingsServer.CancelBooking),
		unary(method("RegisterResource"), BookingsServer.RegisterResource),
		unary(method("FindBooking"), BookingsServer.FindBooking),
		unary(method("FindBookingsByResource"), BookingsServer.FindBookingsByResource),
	},
	Metadata: descriptor.ParentFile().Path(),
}

func method(name protoreflect.Name) protoreflect.MethodDescriptor {
	md := descriptor.Methods().ByName(name)
	if md == nil {
		panic("rpc: BookingsService declares no method " + string(name))
	}
	return md
}

func unary[Req any, PReq interface {
	*Req
	proto.Message
}, Resp proto.Message](md protoreflect.MethodDescriptor, call func(BookingsServer, context.Context, PReq) (Resp, error)) grpc.MethodDesc {
	name := string(md.Name())
	fullMethod := FullMethod(name)
	invoke := func(server BookingsServer, ctx context.Context, req PReq) (any, error) {
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
			server := srv.(BookingsServer)
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

// Server realizes BookingsServer over the bookings use cases.
type Server struct {
	Service application.Service
}

func (s Server) ReserveBooking(ctx context.Context, req *servicev1.ReserveBookingRequest) (*servicev1.ReserveBookingResponse, error) {
	booking, err := bookingID(req.GetBookingId())
	if err != nil {
		return nil, err
	}
	if !bookingIDFormat.MatchString(req.GetResourceId()) {
		return nil, status.Error(codes.InvalidArgument, "resource_id must have 1 to 128 characters of [A-Za-z0-9._:-]")
	}
	if q := req.GetQuantity(); q < minQuantity || q > maxQuantity {
		return nil, status.Error(codes.InvalidArgument, "quantity must be between 1 and 100")
	}
	if _, err := executionOf(ctx); err != nil {
		return nil, err
	}
	out, err := s.Service.ReserveBooking(ctx, application.ReserveBooking{
		BookingID:  booking,
		ResourceID: domain.ResourceID(req.GetResourceId()),
		Quantity:   int(req.GetQuantity()),
	})
	if err != nil {
		return nil, statusOf(err)
	}
	if rejection, refused := out.Rejection(); refused {
		return &servicev1.ReserveBookingResponse{Result: &servicev1.ReserveBookingResponse_Rejection{Rejection: rejectionOf(rejection)}}, nil
	}
	return &servicev1.ReserveBookingResponse{Result: &servicev1.ReserveBookingResponse_Reserved{
		Reserved: &servicev1.Reserved{BookingId: string(out.Response().BookingID)},
	}}, nil
}

func (s Server) CancelBooking(ctx context.Context, req *servicev1.CancelBookingRequest) (*servicev1.CancelBookingResponse, error) {
	booking, err := bookingID(req.GetBookingId())
	if err != nil {
		return nil, err
	}
	if _, err := executionOf(ctx); err != nil {
		return nil, err
	}
	out, err := s.Service.CancelBooking(ctx, application.CancelBooking{BookingID: booking})
	if err != nil {
		return nil, statusOf(err)
	}
	if rejection, refused := out.Rejection(); refused {
		return &servicev1.CancelBookingResponse{Result: &servicev1.CancelBookingResponse_Rejection{Rejection: rejectionOf(rejection)}}, nil
	}
	return &servicev1.CancelBookingResponse{Result: &servicev1.CancelBookingResponse_Cancelled{
		Cancelled: &servicev1.Cancelled{BookingId: string(out.Response().BookingID)},
	}}, nil
}

func (s Server) RegisterResource(ctx context.Context, req *servicev1.RegisterResourceRequest) (*servicev1.RegisterResourceResponse, error) {
	resource, err := resourceID(req.GetResourceId())
	if err != nil {
		return nil, err
	}
	if _, err := executionOf(ctx); err != nil {
		return nil, err
	}
	out, err := s.Service.RegisterResource(ctx, application.RegisterResource{Code: domain.ResourceCode(resource)})
	if err != nil {
		return nil, statusOf(err)
	}
	if rejection, refused := out.Rejection(); refused {
		return &servicev1.RegisterResourceResponse{Result: &servicev1.RegisterResourceResponse_Rejection{Rejection: rejectionOf(rejection)}}, nil
	}
	return &servicev1.RegisterResourceResponse{Result: &servicev1.RegisterResourceResponse_Registered{
		Registered: &servicev1.Registered{ResourceId: string(out.Response().Code)},
	}}, nil
}

func (s Server) FindBooking(ctx context.Context, req *servicev1.FindBookingRequest) (*servicev1.FindBookingResponse, error) {
	booking, err := bookingID(req.GetBookingId())
	if err != nil {
		return nil, err
	}
	if _, err := executionOf(ctx); err != nil {
		return nil, err
	}
	snapshot, err := s.Service.FindBooking(ctx, booking)
	if err != nil {
		return nil, statusOf(err)
	}
	return &servicev1.FindBookingResponse{Booking: bookingOf(snapshot)}, nil
}

func (s Server) FindBookingsByResource(ctx context.Context, req *servicev1.FindBookingsByResourceRequest) (*servicev1.FindBookingsByResourceResponse, error) {
	resource, err := resourceID(req.GetResourceId())
	if err != nil {
		return nil, err
	}
	if _, err := executionOf(ctx); err != nil {
		return nil, err
	}
	snapshots, err := s.Service.FindBookingByResource(ctx, domain.ResourceID(resource))
	if err != nil {
		return nil, statusOf(err)
	}
	bookings := make([]*servicev1.Booking, 0, len(snapshots))
	for _, snapshot := range snapshots {
		bookings = append(bookings, bookingOf(snapshot))
	}
	return &servicev1.FindBookingsByResourceResponse{Bookings: bookings}, nil
}

func bookingID(raw string) (domain.BookingID, error) {
	if !bookingIDFormat.MatchString(raw) {
		return "", status.Error(codes.InvalidArgument, "booking_id must have 1 to 128 characters of [A-Za-z0-9._:-]")
	}
	return domain.BookingID(raw), nil
}

func resourceID(raw string) (string, error) {
	if raw == "" || len(raw) > maxResourceID {
		return "", status.Error(codes.InvalidArgument, "resource_id must have 1 to 128 characters")
	}
	return raw, nil
}

func bookingOf(s domain.BookingSnapshot) *servicev1.Booking {
	return &servicev1.Booking{
		BookingId:  string(s.ID),
		ResourceId: string(s.ResourceID),
		Quantity:   int32(s.Quantity),
		Status:     bookingStatus(s.Status),
		ReservedAt: int64(s.ReservedAt),
	}
}

func bookingStatus(s domain.BookingStatus) servicev1.BookingStatus {
	switch s {
	case domain.BookingReservedStatus:
		return servicev1.BookingStatus_BOOKING_STATUS_RESERVED
	case domain.BookingCancelled:
		return servicev1.BookingStatus_BOOKING_STATUS_CANCELLED
	default:
		return servicev1.BookingStatus_BOOKING_STATUS_UNSPECIFIED
	}
}

func rejectionOf(rejection *kernel.Rejection) *servicev1.Rejection {
	return &servicev1.Rejection{Code: string(rejection.Code()), Message: rejection.Message()}
}
