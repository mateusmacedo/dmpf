package domain

import (
	"strconv"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// AddItem is a UPR: it decides over a copy and commits only on Accepted, so a
// rejection leaves the order untouched (DEC-10) and carries no event (DEC-11).
func (o *Order) AddItem(cmd AddItem) (kernel.Accepted[ItemAccepted], *kernel.Rejection) {
	return kernel.DecideOver(o, (*Order).clone, func(next *Order) (kernel.Accepted[ItemAccepted], *kernel.Rejection) {
		if next.status != Open {
			return kernel.Refuse[ItemAccepted](CodeOrderNotOpen, "order is not open")
		}
		attempted := len(next.items) + 1
		if attempted > next.itemLimit {
			return kernel.Refuse[ItemAccepted](CodeOrderItemLimitExceeded, "item limit exceeded",
				kernel.Detail{Key: "limit", Value: strconv.Itoa(next.itemLimit)},
				kernel.Detail{Key: "attempted", Value: strconv.Itoa(attempted)},
			)
		}
		next.items = append(next.items, Item{SKU: cmd.SKU, Quantity: cmd.Quantity})
		return kernel.Accept(
			ItemAccepted{Order: next.id, Items: len(next.items)},
			ItemAdded{Order: next.id, SKU: cmd.SKU, Quantity: cmd.Quantity, At: cmd.At},
		), nil
	})
}
