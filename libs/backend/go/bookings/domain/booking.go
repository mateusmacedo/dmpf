package domain

type Booking struct {
	id         BookingID
	resourceID ResourceID
	quantity   int
	status     BookingStatus
	reservedAt Instant
}

func NewBooking(id BookingID) *Booking {
	return &Booking{id: id}
}

func FromBookingSnapshot(s BookingSnapshot) *Booking {
	return &Booking{
		id:         s.ID,
		resourceID: s.ResourceID,
		quantity:   s.Quantity,
		status:     s.Status,
		reservedAt: s.ReservedAt,
	}
}

type BookingSnapshot struct {
	ID         BookingID
	ResourceID ResourceID
	Quantity   int
	Status     BookingStatus
	ReservedAt Instant
}

func (b *Booking) Snapshot() BookingSnapshot {
	return BookingSnapshot{
		ID:         b.id,
		ResourceID: b.resourceID,
		Quantity:   b.quantity,
		Status:     b.status,
		ReservedAt: b.reservedAt,
	}
}

func (s BookingSnapshot) Equal(other BookingSnapshot) bool {
	return s.ID == other.ID &&
		s.ResourceID == other.ResourceID &&
		s.Quantity == other.Quantity &&
		s.Status == other.Status &&
		s.ReservedAt == other.ReservedAt
}

func (b *Booking) clone() Booking {
	return Booking{
		id:         b.id,
		resourceID: b.resourceID,
		quantity:   b.quantity,
		status:     b.status,
		reservedAt: b.reservedAt,
	}
}
