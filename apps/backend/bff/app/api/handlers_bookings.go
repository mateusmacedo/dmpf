package api

import (
	"net/http"
	"unicode/utf8"

	bookingsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/bookings/service/v1"
)

const (
	maxResourceLength = 128
	maxQuantity       = 100
)

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

// WHY: maxLength in JSON Schema counts code points, so len would refuse a value
// the contract accepts as soon as it carries a non-ASCII character.
func withinResourceLength(value string) bool {
	return value != "" && utf8.RuneCountInString(value) <= maxResourceLength
}

func (h handlers) reserveBooking(w http.ResponseWriter, r *http.Request) {
	var req reserveBookingRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeRejection(r, w, http.StatusBadRequest, "malformed-body", "the body is not the ReserveRequest of the contract")
		return
	}
	if !idFormat.MatchString(req.BookingID) || !idFormat.MatchString(req.ResourceID) || req.Quantity < 1 || req.Quantity > maxQuantity {
		writeRejection(r, w, http.StatusBadRequest, "invalid-request", "bookingId and resourceId must have 1 to 128 characters of [A-Za-z0-9._:-] and quantity must be 1 to 100")
		return
	}

	resp, err := h.bookings.ReserveBooking(r.Context(), &bookingsv1.ReserveBookingRequest{BookingId: req.BookingID, ResourceId: req.ResourceID, Quantity: req.Quantity})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	switch result := resp.GetResult().(type) {
	case *bookingsv1.ReserveBookingResponse_Reserved:
		writeJSON(r, w, http.StatusCreated, bookingIDResponse{BookingID: result.Reserved.GetBookingId()})
	case *bookingsv1.ReserveBookingResponse_Rejection:
		writeRefusal(r, w, result.Rejection)
	default:
		writeFailure(w, r, errMissingResult)
	}
}

func (h handlers) cancelBooking(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	resp, err := h.bookings.CancelBooking(r.Context(), &bookingsv1.CancelBookingRequest{BookingId: id})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	switch result := resp.GetResult().(type) {
	case *bookingsv1.CancelBookingResponse_Cancelled:
		writeJSON(r, w, http.StatusOK, bookingIDResponse{BookingID: result.Cancelled.GetBookingId()})
	case *bookingsv1.CancelBookingResponse_Rejection:
		writeRefusal(r, w, result.Rejection)
	default:
		writeFailure(w, r, errMissingResult)
	}
}

func (h handlers) registerResource(w http.ResponseWriter, r *http.Request) {
	var req registerResourceRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeRejection(r, w, http.StatusBadRequest, "malformed-body", "the body is not the RegisterRequest of the contract")
		return
	}
	if !withinResourceLength(req.Code) {
		writeRejection(r, w, http.StatusBadRequest, "invalid-request", "code must have 1 to 128 characters")
		return
	}

	resp, err := h.bookings.RegisterResource(r.Context(), &bookingsv1.RegisterResourceRequest{ResourceId: req.Code})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	switch result := resp.GetResult().(type) {
	case *bookingsv1.RegisterResourceResponse_Registered:
		writeJSON(r, w, http.StatusCreated, registeredResource{Code: result.Registered.GetResourceId()})
	case *bookingsv1.RegisterResourceResponse_Rejection:
		writeRefusal(r, w, result.Rejection)
	default:
		writeFailure(w, r, errMissingResult)
	}
}

func (h handlers) findBooking(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	resp, err := h.bookings.FindBooking(r.Context(), &bookingsv1.FindBookingRequest{BookingId: id})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	view, known := viewOf(resp.GetBooking())
	if !known {
		writeFailure(w, r, errUnknownStatus)
		return
	}
	writeJSON(r, w, http.StatusOK, view)
}

func (h handlers) findBookingByResource(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resourceId")
	if !withinResourceLength(resource) {
		writeRejection(r, w, http.StatusBadRequest, "invalid-request", "resourceId must have 1 to 128 characters")
		return
	}
	resp, err := h.bookings.FindBookingsByResource(r.Context(), &bookingsv1.FindBookingsByResourceRequest{ResourceId: resource})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	views := make([]bookingView, 0, len(resp.GetBookings()))
	for _, booking := range resp.GetBookings() {
		view, known := viewOf(booking)
		if !known {
			writeFailure(w, r, errUnknownStatus)
			return
		}
		views = append(views, view)
	}
	writeJSON(r, w, http.StatusOK, views)
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
