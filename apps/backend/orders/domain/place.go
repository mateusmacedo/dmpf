package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Place is a UPR: same decide-over-copy shape as AddItem.
func (o *Order) Place(cmd PlaceOrder) (kernel.Accepted[PlacedResponse], *kernel.Rejection) {
	return kernel.DecideOver(o, (*Order).clone, func(next *Order) (kernel.Accepted[PlacedResponse], *kernel.Rejection) {
		if next.status != Open {
			return kernel.Refuse[PlacedResponse](CodeOrderNotOpen, "order is not open")
		}
		if len(next.items) == 0 {
			return kernel.Refuse[PlacedResponse](CodeOrderEmpty, "order has no items")
		}
		next.status = Placed
		return kernel.Accept(
			PlacedResponse{Order: next.id},
			OrderPlaced{Order: next.id, Items: len(next.items), At: cmd.At},
		), nil
	})
}
