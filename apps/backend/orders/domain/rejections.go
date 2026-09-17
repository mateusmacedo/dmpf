package domain

import "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Rejection codes of the orders example, in the "context/reason" form (FND-03 §5.3).
const (
	// CodeItemLimitExceeded: adding the item would exceed the order's item limit.
	CodeItemLimitExceeded domain.Code = "orders/item-limit-exceeded"
	// CodeEmptyOrder: an order with no items cannot be placed.
	CodeEmptyOrder domain.Code = "orders/empty-order"
	// CodeOrderNotOpen: the order is no longer open to the requested operation.
	CodeOrderNotOpen domain.Code = "orders/order-not-open"
)
