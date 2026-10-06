package rpc

import (
	"context"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	servicev1 "github.com/mateusmacedo/dmpf/apps/backend/bookings/contract/gen/go/company/bookings/service/v1"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	minQuantity = 1
	maxQuantity = 100
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
func Methods() []string { return kernelgrpc.MethodNames(descriptor) }

// Commands lists the methods that require an idempotency key (IDM-01); the
// queries neither require nor read one.
func Commands() []string {
	return []string{"ReserveBooking", "CancelBooking", "RegisterResource"}
}

// FullMethod is the wire name of a method of the service: /<service>/<method>.
func FullMethod(name string) string { return kernelgrpc.FullMethod(ServiceName, name) }

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
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "ReserveBooking"), BookingsServer.ReserveBooking),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "CancelBooking"), BookingsServer.CancelBooking),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RegisterResource"), BookingsServer.RegisterResource),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindBooking"), BookingsServer.FindBooking),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindBookingsByResource"), BookingsServer.FindBookingsByResource),
	},
	Metadata: descriptor.ParentFile().Path(),
}

// Server realizes BookingsServer over the bookings use cases.
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
	return command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ReservedResponse], error) {
			return s.Service.ReserveBooking(ctx, application.ReserveBooking{
				BookingID:  booking,
				ResourceID: domain.ResourceID(req.GetResourceId()),
				Quantity:   int(req.GetQuantity()),
			})
		},
		reserveBookingRefused,
		bookingReservedOf)
}

func (s Server) CancelBooking(ctx context.Context, req *servicev1.CancelBookingRequest) (*servicev1.CancelBookingResponse, error) {
	booking, err := bookingID(req.GetBookingId())
	if err != nil {
		return nil, err
	}
	return command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.CancelledResponse], error) {
			return s.Service.CancelBooking(ctx, application.CancelBooking{BookingID: booking})
		},
		cancelBookingRefused,
		bookingCancelledOf)
}

func (s Server) RegisterResource(ctx context.Context, req *servicev1.RegisterResourceRequest) (*servicev1.RegisterResourceResponse, error) {
	resource, err := resourceID(req.GetResourceId())
	if err != nil {
		return nil, err
	}
	return command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RegisteredResponse], error) {
			return s.Service.RegisterResource(ctx, application.RegisterResource{Code: domain.ResourceCode(resource)})
		},
		registerResourceRefused,
		resourceRegisteredOf)
}

func reserveBookingRefused(r *servicev1.Rejection) *servicev1.ReserveBookingResponse {
	return &servicev1.ReserveBookingResponse{Result: &servicev1.ReserveBookingResponse_Rejection{Rejection: r}}
}

func bookingReservedOf(reserved domain.ReservedResponse) *servicev1.ReserveBookingResponse {
	return &servicev1.ReserveBookingResponse{Result: &servicev1.ReserveBookingResponse_Reserved{
		Reserved: &servicev1.Reserved{BookingId: string(reserved.BookingID)},
	}}
}

func cancelBookingRefused(r *servicev1.Rejection) *servicev1.CancelBookingResponse {
	return &servicev1.CancelBookingResponse{Result: &servicev1.CancelBookingResponse_Rejection{Rejection: r}}
}

func bookingCancelledOf(cancelled domain.CancelledResponse) *servicev1.CancelBookingResponse {
	return &servicev1.CancelBookingResponse{Result: &servicev1.CancelBookingResponse_Cancelled{
		Cancelled: &servicev1.Cancelled{BookingId: string(cancelled.BookingID)},
	}}
}

func registerResourceRefused(r *servicev1.Rejection) *servicev1.RegisterResourceResponse {
	return &servicev1.RegisterResourceResponse{Result: &servicev1.RegisterResourceResponse_Rejection{Rejection: r}}
}

func resourceRegisteredOf(registered domain.RegisteredResponse) *servicev1.RegisterResourceResponse {
	return &servicev1.RegisterResourceResponse{Result: &servicev1.RegisterResourceResponse_Registered{
		Registered: &servicev1.Registered{ResourceId: string(registered.Code)},
	}}
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
		return nil, kernelgrpc.StatusOf(err)
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
		return nil, kernelgrpc.StatusOf(err)
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
	if !bookingIDFormat.MatchString(raw) {
		return "", status.Error(codes.InvalidArgument, "resource_id must have 1 to 128 characters of [A-Za-z0-9._:-]")
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
	case domain.Reserved:
		return servicev1.BookingStatus_BOOKING_STATUS_RESERVED
	case domain.Cancelled:
		return servicev1.BookingStatus_BOOKING_STATUS_CANCELLED
	default:
		return servicev1.BookingStatus_BOOKING_STATUS_UNSPECIFIED
	}
}

func rejectionOf(rejection *kernel.Rejection) *servicev1.Rejection {
	return &servicev1.Rejection{Code: string(rejection.Code()), Message: rejection.Message()}
}
