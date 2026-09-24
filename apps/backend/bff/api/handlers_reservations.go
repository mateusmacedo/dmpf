package api

import (
	"net/http"

	reservationsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/reservations/service/v1"
)

type reserveRequest struct {
	Items int32 `json:"items"`
}

type reserved struct {
	Order string `json:"order"`
	Items int32  `json:"items"`
}

type canceled struct {
	Order string `json:"order"`
}

type reservationView struct {
	Order  string `json:"order"`
	Status string `json:"status"`
	Items  int32  `json:"items"`
}

var reservationStatuses = map[reservationsv1.ReservationStatus]string{
	reservationsv1.ReservationStatus_RESERVATION_STATUS_CONFIRMED: "confirmed",
	reservationsv1.ReservationStatus_RESERVATION_STATUS_CANCELED:  "canceled",
}

func (h handlers) reserve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "order_id")
	if !ok {
		return
	}
	var req reserveRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeRejection(r, w, http.StatusBadRequest, "malformed-body", "the body is not the ReserveRequest of the contract")
		return
	}
	if req.Items < 1 {
		writeRejection(r, w, http.StatusBadRequest, "invalid-request", "items must be at least 1")
		return
	}

	resp, err := h.reservations.Reserve(r.Context(), &reservationsv1.ReserveRequest{OrderId: id, ItemCount: req.Items})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	switch result := resp.GetResult().(type) {
	case *reservationsv1.ReserveResponse_Reserved:
		writeJSON(r, w, http.StatusOK, reserved{Order: result.Reserved.GetOrderId(), Items: result.Reserved.GetItemCount()})
	case *reservationsv1.ReserveResponse_Rejection:
		writeRefusal(r, w, result.Rejection)
	default:
		writeFailure(w, r, errMissingResult)
	}
}

func (h handlers) cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "order_id")
	if !ok {
		return
	}
	resp, err := h.reservations.Cancel(r.Context(), &reservationsv1.CancelRequest{OrderId: id})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	switch result := resp.GetResult().(type) {
	case *reservationsv1.CancelResponse_Canceled:
		writeJSON(r, w, http.StatusOK, canceled{Order: result.Canceled.GetOrderId()})
	case *reservationsv1.CancelResponse_Rejection:
		writeRefusal(r, w, result.Rejection)
	default:
		writeFailure(w, r, errMissingResult)
	}
}

func (h handlers) findReservation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "order_id")
	if !ok {
		return
	}
	resp, err := h.reservations.FindReservation(r.Context(), &reservationsv1.FindReservationRequest{OrderId: id})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	reservation := resp.GetReservation()
	status, known := reservationStatuses[reservation.GetStatus()]
	if !known {
		writeFailure(w, r, errUnknownStatus)
		return
	}
	writeJSON(r, w, http.StatusOK, reservationView{Order: reservation.GetOrderId(), Status: status, Items: reservation.GetItemCount()})
}
