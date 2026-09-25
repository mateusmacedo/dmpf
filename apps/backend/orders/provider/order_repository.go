package provider

import (
	"encoding/json"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// orderState is the snapshot column. The JSON names are fixed here so a rename
// in the domain does not rewrite stored rows; the id lives in its own column.
type orderState struct {
	Status    int         `json:"status"`
	ItemLimit int         `json:"itemLimit"`
	Items     []itemState `json:"items"`
}

type itemState struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

// orderTable declares how the Order snapshot maps to columns. The statements,
// and with them the tenant predicate, are the kernel's: a read or write written
// here could not omit the scope even by mistake (IDN-14). No query filters an
// order by anything but its id, so the whole state is the snapshot.
var orderTable = postgres.Table[domain.OrderID, domain.Snapshot]{
	Name:     "orders",
	IDColumn: "order_id",
	Columns:  []string{"snapshot"},

	Encode: func(s domain.Snapshot) ([]any, error) {
		state := orderState{Status: int(s.Status), ItemLimit: s.ItemLimit, Items: make([]itemState, 0, len(s.Items))}
		for _, item := range s.Items {
			state.Items = append(state.Items, itemState{SKU: string(item.SKU), Quantity: item.Quantity})
		}
		raw, err := json.Marshal(state)
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
		var state orderState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.Snapshot{}, err
		}
		snapshot := domain.Snapshot{Status: domain.Status(state.Status), ItemLimit: state.ItemLimit}
		for _, item := range state.Items {
			snapshot.Items = append(snapshot.Items, domain.Item{SKU: domain.SKU(item.SKU), Quantity: item.Quantity})
		}
		return snapshot, nil
	},

	WithID: func(s domain.Snapshot, id domain.OrderID) domain.Snapshot {
		s.ID = id
		return s
	},
}

// NewOrderRepository binds the repository to an open transaction, because every read
// and write of the aggregate has to run on the same transaction as the outbox
// row (UOW-01).
func NewOrderRepository(tx *postgres.Tx) ports.Repository[domain.OrderID, domain.Snapshot] {
	return orderTable.Repository(tx)
}
