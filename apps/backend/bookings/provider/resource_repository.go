package provider

import (
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

type resourceState struct {
	RegisteredAt int64 `json:"registeredAt"`
}

// resourceTable declares how the Resource snapshot maps to columns, under the
// same kernel statements as bookingTable (IDN-14). No query filters a resource
// by anything but its id, so the whole state is the snapshot.
var resourceTable = postgres.SnapshotTable("resources", "resource_id",
	func(s domain.ResourceSnapshot) resourceState {
		return resourceState{RegisteredAt: int64(s.RegisteredAt)}
	},
	func(state resourceState) domain.ResourceSnapshot {
		return domain.ResourceSnapshot{RegisteredAt: domain.Instant(state.RegisteredAt)}
	},
	func(s domain.ResourceSnapshot, code domain.ResourceCode) domain.ResourceSnapshot {
		s.Code = code
		return s
	},
)

func NewResourceRepository(tx *postgres.Tx) ports.Repository[domain.ResourceCode, domain.ResourceSnapshot] {
	return resourceTable.Repository(tx)
}
