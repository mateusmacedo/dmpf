package provider

import (
	"encoding/json"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// ordersTable declares how the Order snapshot maps to columns. The statements,
// and with them the tenant predicate, are the kernel's: a read or write written
// here could not omit the scope even by mistake (IDN-14).
var ordersTable = postgres.Table[domain.OrderID, domain.Snapshot]{
	Name:     "dmpf_example_orders",
	IDColumn: "order_id",
	Columns:  []string{"snapshot"},

	Encode: func(s domain.Snapshot) ([]any, error) {
		raw, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		return []any{raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.Snapshot, error) {
		var raw []byte
		if err := scan(&raw); err != nil {
			return domain.Snapshot{}, err
		}
		var snapshot domain.Snapshot
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			return domain.Snapshot{}, err
		}
		return snapshot, nil
	},
}

// NewRepository binds the repository to an open transaction, because every read
// and write of the aggregate has to run on the same transaction as the outbox
// row (UOW-01).
func NewRepository(tx *postgres.Tx) ports.Repository[domain.OrderID, domain.Snapshot] {
	return ordersTable.Repository(tx)
}

// NewReader serves the read side without the write side (UOW-11): it takes the
// pool because a query must not open a transaction.
func NewReader(pool postgres.ReadPool) ports.Reader[domain.OrderID, domain.Snapshot] {
	return ordersTable.Reader(pool)
}
