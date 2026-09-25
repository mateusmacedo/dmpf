package domain

import "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Place is a UPR: same decide-over-copy shape as AddItem.
func (o *Order) Place(cmd PlaceOrder) (domain.Accepted[PlacedResponse], *domain.Rejection) {
	next := o.clone()
	if next.status != Open {
		return domain.Accepted[PlacedResponse]{}, domain.Reject(CodeOrderNotOpen, "order is not open")
	}
	if len(next.items) == 0 {
		return domain.Accepted[PlacedResponse]{}, domain.Reject(CodeOrderEmpty, "order has no items")
	}
	next.status = Placed
	*o = next
	return domain.Accept(
		PlacedResponse{Order: o.id},
		OrderPlaced{Order: o.id, Items: len(o.items), At: cmd.At},
	), nil
}
