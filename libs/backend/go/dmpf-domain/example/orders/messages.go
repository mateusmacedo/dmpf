package orders

import dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"

type OrderID string

type SKU string

type Item struct {
	SKU      SKU
	Quantity int
}

type Status int

const (
	Open Status = iota
	Placed
)

// Instant is a point in time already resolved by the application service and
// carried as a value (RFC §9.3): the domain never consults a clock.
type Instant int64

// Commands: imperative verbs over the aggregate (FND-03 §5.3).
type AddItem struct {
	SKU      SKU
	Quantity int
	At       Instant
}

type PlaceOrder struct {
	At Instant
}

// Domain events: facts in the past tense, without version or transport
// (MSG-N01..N03). All fields are comparable values (see dmpfdomain doc).
type ItemAdded struct {
	Order    OrderID
	SKU      SKU
	Quantity int
	At       Instant
}

func (ItemAdded) EventName() string { return "orders.item-added" }

type OrderPlaced struct {
	Order OrderID
	Items int
	At    Instant
}

func (OrderPlaced) EventName() string { return "orders.order-placed" }

// Responses of each UPR.
type ItemAccepted struct {
	Order OrderID
	Items int
}

type PlacedResponse struct {
	Order OrderID
}

var _ dmpfdomain.DomainEvent = ItemAdded{}
var _ dmpfdomain.DomainEvent = OrderPlaced{}
