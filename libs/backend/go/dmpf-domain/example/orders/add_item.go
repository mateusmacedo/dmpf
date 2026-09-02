package orders

import (
	"strconv"

	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
)

// AddItem is a UPR: it decides over a copy and commits only on Accepted, so a
// rejection leaves the order untouched (DEC-10) and carries no event (DEC-11).
func (o *Order) AddItem(cmd AddItem) (dmpfdomain.Accepted[ItemAccepted], *dmpfdomain.Rejection) {
	next := o.clone()
	if next.status != Open {
		return dmpfdomain.Accepted[ItemAccepted]{}, dmpfdomain.Reject(CodeOrderNotOpen, "order is not open")
	}
	attempted := len(next.items) + 1
	if attempted > next.itemLimit {
		return dmpfdomain.Accepted[ItemAccepted]{}, dmpfdomain.Reject(CodeItemLimitExceeded, "item limit exceeded",
			dmpfdomain.Detail{Key: "limit", Value: strconv.Itoa(next.itemLimit)},
			dmpfdomain.Detail{Key: "attempted", Value: strconv.Itoa(attempted)},
		)
	}
	next.items = append(next.items, Item{SKU: cmd.SKU, Quantity: cmd.Quantity})
	*o = next
	return dmpfdomain.Accept(
		ItemAccepted{Order: o.id, Items: len(o.items)},
		ItemAdded{Order: o.id, SKU: cmd.SKU, Quantity: cmd.Quantity, At: cmd.At},
	), nil
}
