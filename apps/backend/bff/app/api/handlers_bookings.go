package api

import (
	"net/http"

	bookingsv1 "github.com/mateusmacedo/dmpf/apps/backend/bookings/contract/gen/go/company/bookings/service/v1"
)

const maxQuantity = 100

type reserveBookingRequest struct {
	BookingID  string `json:"bookingId"`
	ResourceID string `json:"resourceId"`
	Quantity   int32  `json:"quantity"`
}

type bookingIDResponse struct {
	BookingID string `json:"bookingId"`
}

type registerResourceRequest struct {
	Code string `json:"code"`
}

type registeredResource struct {
	Code string `json:"code"`
}

type bookingView struct {
	ID         string `json:"id"`
	ResourceID string `json:"resourceId"`
	Quantity   int32  `json:"quantity"`
	Status     string `json:"status"`
	ReservedAt int64  `json:"reservedAt"`
}

var bookingStatuses = map[bookingsv1.BookingStatus]string{
	bookingsv1.BookingStatus_BOOKING_STATUS_RESERVED:  "reserved",
	bookingsv1.BookingStatus_BOOKING_STATUS_CANCELLED: "cancelled",
}

func (h handlers) reserveBooking() http.HandlerFunc {
	return endpoint(
		fromBody("ReserveRequest",
			"bookingId and resourceId must have 1 to 128 characters of [A-Za-z0-9._:-] and quantity must be 1 to 100",
			validReserveBooking, reserveBookingRequestOf),
		h.bookings.ReserveBooking,
		outcome(http.StatusCreated, bookingReservedOf))
}

func validReserveBooking(b reserveBookingRequest) bool {
	return idFormat.MatchString(b.BookingID) && idFormat.MatchString(b.ResourceID) && b.Quantity >= 1 && b.Quantity <= maxQuantity
}

func reserveBookingRequestOf(b reserveBookingRequest) *bookingsv1.ReserveBookingRequest {
	return &bookingsv1.ReserveBookingRequest{BookingId: b.BookingID, ResourceId: b.ResourceID, Quantity: b.Quantity}
}

func bookingReservedOf(resp *bookingsv1.ReserveBookingResponse) (any, refusal) {
	switch result := resp.GetResult().(type) {
	case *bookingsv1.ReserveBookingResponse_Reserved:
		return bookingIDResponse{BookingID: result.Reserved.GetBookingId()}, nil
	case *bookingsv1.ReserveBookingResponse_Rejection:
		return nil, result.Rejection
	default:
		return nil, nil
	}
}

func (h handlers) cancelBooking() http.HandlerFunc {
	return endpoint(fromPath("id", cancelBookingRequest), h.bookings.CancelBooking, outcome(http.StatusOK, bookingCancelledOf))
}

func cancelBookingRequest(id string) *bookingsv1.CancelBookingRequest {
	return &bookingsv1.CancelBookingRequest{BookingId: id}
}

func bookingCancelledOf(resp *bookingsv1.CancelBookingResponse) (any, refusal) {
	switch result := resp.GetResult().(type) {
	case *bookingsv1.CancelBookingResponse_Cancelled:
		return bookingIDResponse{BookingID: result.Cancelled.GetBookingId()}, nil
	case *bookingsv1.CancelBookingResponse_Rejection:
		return nil, result.Rejection
	default:
		return nil, nil
	}
}

func (h handlers) registerResource() http.HandlerFunc {
	return endpoint(
		fromBody("RegisterRequest", "code must have 1 to 128 characters of [A-Za-z0-9._:-]",
			validRegisterResource, registerResourceRequestOf),
		h.bookings.RegisterResource,
		outcome(http.StatusCreated, resourceRegisteredOf))
}

func validRegisterResource(b registerResourceRequest) bool { return idFormat.MatchString(b.Code) }

func registerResourceRequestOf(b registerResourceRequest) *bookingsv1.RegisterResourceRequest {
	return &bookingsv1.RegisterResourceRequest{ResourceId: b.Code}
}

func resourceRegisteredOf(resp *bookingsv1.RegisterResourceResponse) (any, refusal) {
	switch result := resp.GetResult().(type) {
	case *bookingsv1.RegisterResourceResponse_Registered:
		return registeredResource{Code: result.Registered.GetResourceId()}, nil
	case *bookingsv1.RegisterResourceResponse_Rejection:
		return nil, result.Rejection
	default:
		return nil, nil
	}
}

func (h handlers) findBooking() http.HandlerFunc {
	return endpoint(fromPath("id", findBookingRequest), h.bookings.FindBooking, view(bookingViewOf))
}

func findBookingRequest(id string) *bookingsv1.FindBookingRequest {
	return &bookingsv1.FindBookingRequest{BookingId: id}
}

func bookingViewOf(resp *bookingsv1.FindBookingResponse) (any, bool) {
	return viewOf(resp.GetBooking())
}

func (h handlers) findBookingByResource() http.HandlerFunc {
	return endpoint(fromQuery("resourceId", findBookingsByResourceRequest), h.bookings.FindBookingsByResource, view(bookingViewsOf))
}

func findBookingsByResourceRequest(resource string) *bookingsv1.FindBookingsByResourceRequest {
	return &bookingsv1.FindBookingsByResourceRequest{ResourceId: resource}
}

func bookingViewsOf(resp *bookingsv1.FindBookingsByResourceResponse) (any, bool) {
	views := make([]bookingView, 0, len(resp.GetBookings()))
	for _, booking := range resp.GetBookings() {
		v, known := viewOf(booking)
		if !known {
			return nil, false
		}
		views = append(views, v)
	}
	return views, true
}

func viewOf(b *bookingsv1.Booking) (bookingView, bool) {
	status, known := bookingStatuses[b.GetStatus()]
	return bookingView{
		ID:         b.GetBookingId(),
		ResourceID: b.GetResourceId(),
		Quantity:   b.GetQuantity(),
		Status:     status,
		ReservedAt: b.GetReservedAt(),
	}, known
}
