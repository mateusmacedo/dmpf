package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// OrderID identifies the aggregate; it arrives already generated (RFC §9.3).
type OrderID string

// SKU identifies an item in the domain's own vocabulary, not a catalog key.
type SKU string

// Item is one line of the order.
type Item struct {
	SKU      SKU
	Quantity int
}

// Status is the lifecycle state of the order.
type Status int

const (
	// Open accepts items and can be placed.
	Open Status = iota
	// Placed is terminal for this example: no item can be added afterwards.
	Placed
)

// Instant is a point in time in Unix seconds, resolved by the application
// service and carried as a value (RFC §9.3): the domain never consults a clock.
type Instant int64

// AddItem is the command that asks the order to take one more item (FND-03 §5.3).
type AddItem struct {
	SKU      SKU
	Quantity int
	At       Instant
}

// PlaceOrder is the command that asks the order to be placed.
type PlaceOrder struct {
	At Instant
}

// ItemAdded is the domain event of an item accepted into the order (MSG-N01).
type ItemAdded struct {
	Order    OrderID
	SKU      SKU
	Quantity int
	At       Instant
}

// EventName is stable and carries no version or transport (MSG-N02, MSG-N03).
func (ItemAdded) EventName() string { return "orders.item-added" }

// OrderPlaced is the domain event of an order placed with Items lines.
type OrderPlaced struct {
	Order OrderID
	Items int
	At    Instant
}

// EventName is stable and carries no version or transport (MSG-N02, MSG-N03).
func (OrderPlaced) EventName() string { return "orders.order-placed" }

// ItemAccepted is the response of AddItem: the order and its item count.
type ItemAccepted struct {
	Order OrderID
	Items int
}

// PlacedResponse is the response of Place. It is not named Placed because that
// name is the Status constant.
type PlacedResponse struct {
	Order OrderID
}

var _ kernel.DomainEvent = ItemAdded{}
var _ kernel.DomainEvent = OrderPlaced{}
