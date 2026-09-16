package orderspg

import (
	"fmt"

	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	orderPlacedType = "com.company.orders.order-placed.v1"
	itemAddedType   = "com.company.orders.item-added.v1"
)

// Mapper is the orders bounded context's translation from domain event to
// contract. It carries no state: the mapping is a property of the context, not
// of a request.
type Mapper struct{}

// The switch is exhaustive over the aggregate's two events, and its default is
// the point where an event without a contract is caught — before Enqueue can
// write a row nobody would know how to publish.
func (Mapper) Map(event domain.DomainEvent) (postgres.Mapped, error) {
	switch e := event.(type) {
	case orders.OrderPlaced:
		return postgres.Mapped{
			Message: &eventv1.OrderPlaced{OrderId: string(e.Order), ItemCount: int32(e.Items)},
			Type:    orderPlacedType,
		}, nil
	case orders.ItemAdded:
		return postgres.Mapped{
			Message: &eventv1.ItemAdded{
				OrderId:  string(e.Order),
				Sku:      string(e.SKU),
				Quantity: int32(e.Quantity),
			},
			Type: itemAddedType,
		}, nil
	default:
		return postgres.Mapped{}, fmt.Errorf("%w: %s", postgres.ErrUnmappedEvent, event.EventName())
	}
}
