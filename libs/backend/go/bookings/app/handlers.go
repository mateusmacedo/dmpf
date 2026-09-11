package bookingsapp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"

	bookingsapplication "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/application"
	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

const maxBodyBytes = 1 << 16

var idFormat = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

type Handlers struct{ Service bookingsapplication.Service }

type reserveRequest struct {
	BookingID  string `json:"bookingId"`
	ResourceID string `json:"resourceId"`
	Quantity   int    `json:"quantity"`
}

type reservedResponse struct {
	BookingID string `json:"bookingId"`
}

type cancelledResponse struct {
	BookingID string `json:"bookingId"`
}

type registerRequest struct {
	Code string `json:"code"`
}

type registeredResponse struct {
	Code string `json:"code"`
}

type bookingView struct {
	ID         string `json:"id"`
	ResourceID string `json:"resourceId"`
	Quantity   int    `json:"quantity"`
	Status     string `json:"status"`
	ReservedAt int64  `json:"reservedAt"`
}

type rejection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h Handlers) ReserveBooking(w http.ResponseWriter, r *http.Request) {
	var req reserveRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeRejection(w, http.StatusBadRequest, "malformed-body", "invalid request body")
		return
	}
	if !idFormat.MatchString(req.BookingID) || !idFormat.MatchString(req.ResourceID) || req.Quantity < 1 || req.Quantity > 100 {
		writeRejection(w, http.StatusBadRequest, "invalid-request", "bookingId and resourceId must be 1-128 chars, quantity 1-100")
		return
	}
	out, err := h.Service.ReserveBooking(r.Context(), bookingsapplication.Reserve{
		BookingID:  bookingsdomain.BookingID(req.BookingID),
		ResourceID: bookingsdomain.ResourceID(req.ResourceID),
		Quantity:   req.Quantity,
	})
	if err != nil {
		writeFailure(w, err)
		return
	}
	if rej, refused := out.Rejection(); refused {
		writeDomainRejection(w, rej)
		return
	}
	writeJSON(w, http.StatusCreated, reservedResponse{BookingID: string(out.Response().BookingID)})
}

func (h Handlers) CancelBooking(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idFormat.MatchString(id) {
		writeRejection(w, http.StatusBadRequest, "invalid-request", "id must be 1-128 chars of [A-Za-z0-9._:-]")
		return
	}
	out, err := h.Service.CancelBooking(r.Context(), bookingsapplication.Cancel{
		BookingID: bookingsdomain.BookingID(id),
	})
	if err != nil {
		writeFailure(w, err)
		return
	}
	if rej, refused := out.Rejection(); refused {
		writeDomainRejection(w, rej)
		return
	}
	writeJSON(w, http.StatusOK, cancelledResponse{BookingID: string(out.Response().BookingID)})
}

func (h Handlers) RegisterResource(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeRejection(w, http.StatusBadRequest, "malformed-body", "invalid request body")
		return
	}
	if req.Code == "" || len(req.Code) > 128 {
		writeRejection(w, http.StatusBadRequest, "invalid-request", "code must be 1-128 chars")
		return
	}
	out, err := h.Service.RegisterResource(r.Context(), bookingsapplication.Register{
		Code: bookingsdomain.ResourceCode(req.Code),
	})
	if err != nil {
		writeFailure(w, err)
		return
	}
	if rej, refused := out.Rejection(); refused {
		writeDomainRejection(w, rej)
		return
	}
	writeJSON(w, http.StatusCreated, registeredResponse{Code: string(out.Response().Code)})
}

func (h Handlers) FindBooking(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idFormat.MatchString(id) {
		writeRejection(w, http.StatusBadRequest, "invalid-request", "id must be 1-128 chars of [A-Za-z0-9._:-]")
		return
	}
	snapshot, err := h.Service.FindBooking(r.Context(), bookingsdomain.BookingID(id))
	if err != nil {
		writeFailure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewOf(snapshot))
}

func (h Handlers) FindBookingByResource(w http.ResponseWriter, r *http.Request) {
	resID := r.URL.Query().Get("resourceId")
	if resID == "" || len(resID) > 128 {
		writeRejection(w, http.StatusBadRequest, "invalid-request", "resourceId query param required, 1-128 chars")
		return
	}
	snapshots, err := h.Service.FindBookingByResource(r.Context(), bookingsdomain.ResourceID(resID))
	if err != nil {
		writeFailure(w, err)
		return
	}
	views := make([]bookingView, 0, len(snapshots))
	for _, s := range snapshots {
		views = append(views, viewOf(s))
	}
	writeJSON(w, http.StatusOK, views)
}

func viewOf(s bookingsdomain.BookingSnapshot) bookingView {
	return bookingView{
		ID:         string(s.ID),
		ResourceID: string(s.ResourceID),
		Quantity:   s.Quantity,
		Status:     statusOf(s.Status),
		ReservedAt: int64(s.ReservedAt),
	}
}

func statusOf(s bookingsdomain.BookingStatus) string {
	switch s {
	case bookingsdomain.BookingReservedStatus:
		return "reserved"
	case bookingsdomain.BookingCancelled:
		return "cancelled"
	default:
		return "new"
	}
}

func decodeBody(w http.ResponseWriter, r *http.Request, into any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("bookingsapp: trailing data after request body")
	}
	return nil
}

func writeDomainRejection(w http.ResponseWriter, rej *dmpfdomain.Rejection) {
	writeJSON(w, http.StatusUnprocessableEntity, rejection{Code: string(rej.Code()), Message: rej.Error()})
}

func writeRejection(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, rejection{Code: code, Message: message})
}

func writeFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dmpfports.ErrNotFound):
		writeRejection(w, http.StatusNotFound, "not-found", "aggregate not found")
	default:
		writeRejection(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// compile-time check
var _ dmpfapplication.Outcome[bookingsdomain.ReservedResponse]
