package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Place is a UPR: same decide-over-copy shape as AddItem.
func (o *Order) Place(cmd PlaceOrder) (kernel.Accepted[PlacedResponse], *kernel.Rejection) {
	next := o.clone()
	if next.status != Open {
		return kernel.Accepted[PlacedResponse]{}, kernel.Reject(CodeOrderNotOpen, "order is not open")
	}
	if len(next.items) == 0 {
		return kernel.Accepted[PlacedResponse]{}, kernel.Reject(CodeOrderEmpty, "order has no items")
	}
	next.status = Placed
	*o = next
	return kernel.Accept(
		PlacedResponse{Order: o.id},
		OrderPlaced{Order: o.id, Items: len(o.items), At: cmd.At},
	), nil
}
