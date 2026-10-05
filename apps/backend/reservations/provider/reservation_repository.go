package provider

import (
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// reservationState is the snapshot column. The JSON names are fixed here so a
// rename in the domain does not rewrite stored rows; the order is the key.
type reservationState struct {
	Items  int `json:"items"`
	Status int `json:"status"`
}

// reservationTable maps the Reservation snapshot to columns under the kernel
// statements and their tenant predicate (IDN-14). No query filters a reservation
// by anything but its order, so the whole state is the snapshot.
var reservationTable = postgres.SnapshotTable("reservations", "order_id",
	func(s domain.Snapshot) reservationState {
		return reservationState{Items: s.Items, Status: int(s.Status)}
	},
	func(state reservationState) domain.Snapshot {
		return domain.Snapshot{Items: state.Items, Status: domain.Status(state.Status)}
	},
	func(s domain.Snapshot, order domain.OrderID) domain.Snapshot {
		s.Order = order
		return s
	},
)

// NewReservationRepository binds the repository to an open transaction, because every read
// and write of the aggregate has to run on the same transaction as the outbox
// row (UOW-01).
func NewReservationRepository(tx *postgres.Tx) ports.Repository[domain.OrderID, domain.Snapshot] {
	return reservationTable.Repository(tx)
}
