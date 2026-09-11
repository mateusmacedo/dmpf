package orders

import dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"

// Place is a UPR: same decide-over-copy shape as AddItem.
func (o *Order) Place(cmd PlaceOrder) (dmpfdomain.Accepted[PlacedResponse], *dmpfdomain.Rejection) {
	next := o.clone()
	if next.status != Open {
		return dmpfdomain.Accepted[PlacedResponse]{}, dmpfdomain.Reject(CodeOrderNotOpen, "order is not open")
	}
	if len(next.items) == 0 {
		return dmpfdomain.Accepted[PlacedResponse]{}, dmpfdomain.Reject(CodeEmptyOrder, "order has no items")
	}
	next.status = Placed
	*o = next
	return dmpfdomain.Accept(
		PlacedResponse{Order: o.id},
		OrderPlaced{Order: o.id, Items: len(o.items), At: cmd.At},
	), nil
}
