package provider

import (
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

type resourceState struct {
	Name         string `json:"name"`
	RegisteredAt int64  `json:"registeredAt"`
}

// resourceTable declares how the Resource snapshot maps to columns, under the
// same kernel statements as bookingTable (IDN-14). No query filters a resource
// by anything but its id, so the whole state is the snapshot.
var resourceTable = withUniqueName(postgres.SnapshotTable("resources", "resource_id",
	func(s domain.ResourceSnapshot) resourceState {
		return resourceState{Name: string(s.Name), RegisteredAt: int64(s.RegisteredAt)}
	},
	func(state resourceState) domain.ResourceSnapshot {
		return domain.ResourceSnapshot{Name: domain.ResourceName(state.Name), RegisteredAt: kernel.Instant(state.RegisteredAt)}
	},
	func(s domain.ResourceSnapshot, code domain.ResourceCode) domain.ResourceSnapshot {
		s.Code = code
		return s
	},
))

func withUniqueName(table postgres.Table[domain.ResourceCode, domain.ResourceSnapshot]) postgres.Table[domain.ResourceCode, domain.ResourceSnapshot] {
	table.Unique = []postgres.Unique{{
		Name:      "resources_tenant_id_name_key",
		Columns:   []string{"tenant_id", "name"},
		Rejection: domain.RejectResourceDuplicateName,
	}}
	return table
}

func NewResourceRepository(tx *postgres.Tx) ports.Repository[domain.ResourceCode, domain.ResourceSnapshot] {
	return resourceTable.Repository(tx)
}
