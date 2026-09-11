package orders

import dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"

// Rejection codes of the orders example, in the "context/reason" form (FND-03 §5.3).
const (
	// CodeItemLimitExceeded: adding the item would exceed the order's item limit.
	CodeItemLimitExceeded dmpfdomain.Code = "orders/item-limit-exceeded"
	// CodeEmptyOrder: an order with no items cannot be placed.
	CodeEmptyOrder dmpfdomain.Code = "orders/empty-order"
	// CodeOrderNotOpen: the order is no longer open to the requested operation.
	CodeOrderNotOpen dmpfdomain.Code = "orders/order-not-open"
)
