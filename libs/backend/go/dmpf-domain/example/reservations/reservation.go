package reservations

// Reservation is a second example aggregate of the kernel: a single UPR,
// Reserve, that confirms items for an order (FND-04 §7.2).
type Reservation struct {
	order  OrderID
	items  int
	status Status
}

// NewReservation creates a pending reservation for the given order.
func NewReservation(order OrderID) *Reservation {
	return &Reservation{order: order, status: Pending}
}

// FromSnapshot reconstitutes the reservation from persisted state. It is pure
// computation: no UPR, no event, no clock.
func FromSnapshot(s Snapshot) *Reservation {
	return &Reservation{order: s.Order, items: s.Items, status: s.Status}
}

// Snapshot is the observable state of the reservation, used by tests to prove DEC-10.
type Snapshot struct {
	Order  OrderID
	Items  int
	Status Status
}

// Snapshot copies the observable state; mutating the result never reaches the reservation.
func (r *Reservation) Snapshot() Snapshot {
	return Snapshot{Order: r.order, Items: r.items, Status: r.status}
}

// Equal compares structurally, without reflection.
func (s Snapshot) Equal(other Snapshot) bool {
	return s.Order == other.Order && s.Items == other.Items && s.Status == other.Status
}

// clone gives each UPR its own copy: deciding over the copy and committing
// only on Accepted is what keeps a refusal from touching *r (DEC-10).
func (r *Reservation) clone() Reservation {
	return Reservation{order: r.order, items: r.items, status: r.status}
}
