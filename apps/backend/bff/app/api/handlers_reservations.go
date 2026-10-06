package api

import (
	"net/http"

	reservationsv1 "github.com/mateusmacedo/dmpf/apps/backend/reservations/contract/gen/go/company/reservations/service/v1"
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

func (h handlers) reserve() http.HandlerFunc {
	return endpoint(
		fromPathAndBody("order_id", "ReserveRequest", "items must be at least 1", validReserve, reserveRequestOf),
		h.reservations.Reserve,
		outcome(http.StatusOK, reservedOf))
}

func validReserve(b reserveRequest) bool { return b.Items >= 1 }

func reserveRequestOf(id string, b reserveRequest) *reservationsv1.ReserveRequest {
	return &reservationsv1.ReserveRequest{OrderId: id, ItemCount: b.Items}
}

func reservedOf(resp *reservationsv1.ReserveResponse) (any, refusal) {
	switch result := resp.GetResult().(type) {
	case *reservationsv1.ReserveResponse_Reserved:
		return reserved{Order: result.Reserved.GetOrderId(), Items: result.Reserved.GetItemCount()}, nil
	case *reservationsv1.ReserveResponse_Rejection:
		return nil, result.Rejection
	default:
		return nil, nil
	}
}

func (h handlers) cancel() http.HandlerFunc {
	return endpoint(fromPath("order_id", cancelRequest), h.reservations.Cancel, outcome(http.StatusOK, canceledOf))
}

func cancelRequest(id string) *reservationsv1.CancelRequest {
	return &reservationsv1.CancelRequest{OrderId: id}
}

func canceledOf(resp *reservationsv1.CancelResponse) (any, refusal) {
	switch result := resp.GetResult().(type) {
	case *reservationsv1.CancelResponse_Canceled:
		return canceled{Order: result.Canceled.GetOrderId()}, nil
	case *reservationsv1.CancelResponse_Rejection:
		return nil, result.Rejection
	default:
		return nil, nil
	}
}

func (h handlers) findReservation() http.HandlerFunc {
	return endpoint(fromPath("order_id", findReservationRequest), h.reservations.FindReservation, view(reservationViewOf))
}

func findReservationRequest(id string) *reservationsv1.FindReservationRequest {
	return &reservationsv1.FindReservationRequest{OrderId: id}
}

func reservationViewOf(resp *reservationsv1.FindReservationResponse) (any, bool) {
	reservation := resp.GetReservation()
	status, known := reservationStatuses[reservation.GetStatus()]
	return reservationView{Order: reservation.GetOrderId(), Status: status, Items: reservation.GetItemCount()}, known
}
