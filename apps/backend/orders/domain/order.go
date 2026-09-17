package domain

import "slices"

// Order is the example aggregate of the kernel: the subject of FND-03 §8.2 and
// §8.3 transcribed to Go, with two UPRs (AddItem, Place).
type Order struct {
	id        OrderID
	status    Status
	items     []Item
	itemLimit int
}

// NewOrder creates an open order with no items and the given item limit.
func NewOrder(id OrderID, itemLimit int) *Order {
	return &Order{id: id, status: Open, items: []Item{}, itemLimit: itemLimit}
}

// FromSnapshot reconstitutes the order from persisted state, cloning Items so
// aggregate and snapshot never share the backing array (DEC-12). It is pure
// computation: no UPR, no event, no clock.
func FromSnapshot(s Snapshot) *Order {
	return &Order{id: s.ID, status: s.Status, items: slices.Clone(s.Items), itemLimit: s.ItemLimit}
}

// Snapshot is the observable state of the order, used by tests to prove DEC-10.
type Snapshot struct {
	ID        OrderID
	Status    Status
	ItemLimit int
	Items     []Item
}

// Snapshot copies the observable state; mutating the result never reaches the order.
func (o *Order) Snapshot() Snapshot {
	return Snapshot{ID: o.id, Status: o.status, ItemLimit: o.itemLimit, Items: slices.Clone(o.items)}
}

// Equal compares structurally, item by item, without reflection.
func (s Snapshot) Equal(other Snapshot) bool {
	return s.ID == other.ID && s.Status == other.Status && s.ItemLimit == other.ItemLimit &&
		slices.Equal(s.Items, other.Items)
}

// clone gives each UPR its own backing array: deciding over the copy and
// committing only on Accepted is what keeps a refusal from touching *o (DEC-10).
func (o *Order) clone() Order {
	return Order{id: o.id, status: o.status, items: slices.Clone(o.items), itemLimit: o.itemLimit}
}
