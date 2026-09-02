package orders

import dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"

const (
	CodeItemLimitExceeded dmpfdomain.Code = "orders/item-limit-exceeded"
	CodeEmptyOrder        dmpfdomain.Code = "orders/empty-order"
	CodeOrderNotOpen      dmpfdomain.Code = "orders/order-not-open"
)
