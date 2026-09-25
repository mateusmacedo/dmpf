package domain

import (
	"strconv"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// AddItem is a UPR: it decides over a copy and commits only on Accepted, so a
// rejection leaves the order untouched (DEC-10) and carries no event (DEC-11).
func (o *Order) AddItem(cmd AddItem) (domain.Accepted[ItemAccepted], *domain.Rejection) {
	next := o.clone()
	if next.status != Open {
		return domain.Accepted[ItemAccepted]{}, domain.Reject(CodeOrderNotOpen, "order is not open")
	}
	attempted := len(next.items) + 1
	if attempted > next.itemLimit {
		return domain.Accepted[ItemAccepted]{}, domain.Reject(CodeOrderItemLimitExceeded, "item limit exceeded",
			domain.Detail{Key: "limit", Value: strconv.Itoa(next.itemLimit)},
			domain.Detail{Key: "attempted", Value: strconv.Itoa(attempted)},
		)
	}
	next.items = append(next.items, Item{SKU: cmd.SKU, Quantity: cmd.Quantity})
	*o = next
	return domain.Accept(
		ItemAccepted{Order: o.id, Items: len(o.items)},
		ItemAdded{Order: o.id, SKU: cmd.SKU, Quantity: cmd.Quantity, At: cmd.At},
	), nil
}
