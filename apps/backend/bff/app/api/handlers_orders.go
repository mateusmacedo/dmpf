package api

import (
	"net/http"
	"unicode/utf8"

	ordersv1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/service/v1"
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

func (h handlers) addItem() http.HandlerFunc {
	return endpoint(
		fromPathAndBody("id", "AddItemRequest", "sku must have 1 to 128 characters and quantity must be at least 1",
			validAddItem, addItemRequestOf),
		h.orders.AddItem,
		outcome(http.StatusCreated, itemAcceptedOf))
}

// WHY: maxLength in JSON Schema counts code points, so len would refuse a
// SKU the contract accepts as soon as it carries a non-ASCII character.
func validAddItem(b addItemRequest) bool {
	return b.SKU != "" && utf8.RuneCountInString(b.SKU) <= maxSKULength && b.Quantity >= 1
}

func addItemRequestOf(id string, b addItemRequest) *ordersv1.AddItemRequest {
	return &ordersv1.AddItemRequest{OrderId: id, Sku: b.SKU, Quantity: b.Quantity}
}

func itemAcceptedOf(resp *ordersv1.AddItemResponse) (any, refusal) {
	switch result := resp.GetResult().(type) {
	case *ordersv1.AddItemResponse_Accepted:
		return itemAccepted{Order: result.Accepted.GetOrderId(), Items: result.Accepted.GetItemCount()}, nil
	case *ordersv1.AddItemResponse_Rejection:
		return nil, result.Rejection
	default:
		return nil, nil
	}
}

func (h handlers) placeOrder() http.HandlerFunc {
	return endpoint(fromPath("id", placeOrderRequest), h.orders.PlaceOrder, outcome(http.StatusOK, orderPlacedOf))
}

func placeOrderRequest(id string) *ordersv1.PlaceOrderRequest {
	return &ordersv1.PlaceOrderRequest{OrderId: id}
}

func orderPlacedOf(resp *ordersv1.PlaceOrderResponse) (any, refusal) {
	switch result := resp.GetResult().(type) {
	case *ordersv1.PlaceOrderResponse_Placed:
		return orderPlaced{Order: result.Placed.GetOrderId()}, nil
	case *ordersv1.PlaceOrderResponse_Rejection:
		return nil, result.Rejection
	default:
		return nil, nil
	}
}

func (h handlers) findOrder() http.HandlerFunc {
	return endpoint(fromPath("id", findOrderRequest), h.orders.FindOrder, view(orderViewOf))
}

func findOrderRequest(id string) *ordersv1.FindOrderRequest {
	return &ordersv1.FindOrderRequest{OrderId: id}
}

func orderViewOf(resp *ordersv1.FindOrderResponse) (any, bool) {
	order := resp.GetOrder()
	status, known := orderStatuses[order.GetStatus()]
	items := make([]itemView, 0, len(order.GetItems()))
	for _, item := range order.GetItems() {
		items = append(items, itemView{SKU: item.GetSku(), Quantity: item.GetQuantity()})
	}
	return orderView{ID: order.GetOrderId(), Status: status, ItemLimit: order.GetItemLimit(), Items: items}, known
}
