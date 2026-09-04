package orderspg_test

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
	orderspg "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres/example/orders"
)

type strayEvent struct{}

func (strayEvent) EventName() string { return "orders.stray" }

func TestMapperCoversBothDomainEvents(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		event       dmpfdomain.DomainEvent
		wantMessage proto.Message
		wantType    string
	}{
		{
			name:        "order placed",
			event:       orders.OrderPlaced{Order: "o-1001", Items: 3, At: 1_755_432_000},
			wantMessage: &eventv1.OrderPlaced{OrderId: "o-1001", ItemCount: 3},
			wantType:    "com.company.orders.order-placed.v1",
		},
		{
			name:        "item added",
			event:       orders.ItemAdded{Order: "o-1001", SKU: "sku-1", Quantity: 2, At: 1_755_432_000},
			wantMessage: &eventv1.ItemAdded{OrderId: "o-1001", Sku: "sku-1", Quantity: 2},
			wantType:    "com.company.orders.item-added.v1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mapped, err := orderspg.Mapper{}.Map(tc.event)

			if err != nil {
				t.Fatalf("Map() = %v, want nil", err)
			}
			if !proto.Equal(mapped.Message, tc.wantMessage) {
				t.Errorf("Map().Message = %v, want %v", mapped.Message, tc.wantMessage)
			}
			if mapped.Type != tc.wantType {
				t.Errorf("Map().Type = %q, want %q (PTB-03)", mapped.Type, tc.wantType)
			}
		})
	}
}

func TestMapperReportsAnUnmappedEvent(t *testing.T) {
	t.Parallel()

	_, err := orderspg.Mapper{}.Map(strayEvent{})

	if !errors.Is(err, dmpfpostgres.ErrUnmappedEvent) {
		t.Fatalf("Map() = %v, want ErrUnmappedEvent", err)
	}
}
