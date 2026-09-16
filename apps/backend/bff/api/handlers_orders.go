package api

import (
	"net/http"
	"unicode/utf8"

	ordersv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/service/v1"
)

type addItemRequest struct {
	SKU      string `json:"sku"`
	Quantity int32  `json:"quantity"`
}

type itemAccepted struct {
	Order string `json:"order"`
	Items int32  `json:"items"`
}

type orderPlaced struct {
	Order string `json:"order"`
}

type itemView struct {
	SKU      string `json:"sku"`
	Quantity int32  `json:"quantity"`
}

type orderView struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	ItemLimit int32      `json:"itemLimit"`
	Items     []itemView `json:"items"`
}

var orderStatuses = map[ordersv1.OrderStatus]string{
	ordersv1.OrderStatus_ORDER_STATUS_OPEN:   "open",
	ordersv1.OrderStatus_ORDER_STATUS_PLACED: "placed",
}

func (h handlers) addItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var req addItemRequest
	if err := decodeBody(w, r, &req); err != nil {
		writeRejection(r, w, http.StatusBadRequest, "malformed-body", "the body is not the AddItemRequest of the contract")
		return
	}
	// WHY: maxLength in JSON Schema counts code points, so len would refuse a
	// SKU the contract accepts as soon as it carries a non-ASCII character.
	if req.SKU == "" || utf8.RuneCountInString(req.SKU) > maxSKULength || req.Quantity < 1 {
		writeRejection(r, w, http.StatusBadRequest, "invalid-request", "sku must have 1 to 128 characters and quantity must be at least 1")
		return
	}

	resp, err := h.orders.AddItem(r.Context(), &ordersv1.AddItemRequest{OrderId: id, Sku: req.SKU, Quantity: req.Quantity})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	switch result := resp.GetResult().(type) {
	case *ordersv1.AddItemResponse_Accepted:
		writeJSON(r, w, http.StatusCreated, itemAccepted{Order: result.Accepted.GetOrderId(), Items: result.Accepted.GetItemCount()})
	case *ordersv1.AddItemResponse_Rejection:
		writeRefusal(r, w, result.Rejection)
	default:
		writeFailure(w, r, errMissingResult)
	}
}

func (h handlers) placeOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	resp, err := h.orders.PlaceOrder(r.Context(), &ordersv1.PlaceOrderRequest{OrderId: id})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	switch result := resp.GetResult().(type) {
	case *ordersv1.PlaceOrderResponse_Placed:
		writeJSON(r, w, http.StatusOK, orderPlaced{Order: result.Placed.GetOrderId()})
	case *ordersv1.PlaceOrderResponse_Rejection:
		writeRefusal(r, w, result.Rejection)
	default:
		writeFailure(w, r, errMissingResult)
	}
}

func (h handlers) findOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	resp, err := h.orders.FindOrder(r.Context(), &ordersv1.FindOrderRequest{OrderId: id})
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	order := resp.GetOrder()
	status, known := orderStatuses[order.GetStatus()]
	if !known {
		writeFailure(w, r, errUnknownStatus)
		return
	}
	items := make([]itemView, 0, len(order.GetItems()))
	for _, item := range order.GetItems() {
		items = append(items, itemView{SKU: item.GetSku(), Quantity: item.GetQuantity()})
	}
	writeJSON(r, w, http.StatusOK, orderView{ID: order.GetOrderId(), Status: status, ItemLimit: order.GetItemLimit(), Items: items})
}
