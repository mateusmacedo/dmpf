package relay

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/payloadhash"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

const publishableMetadata = `{"correlationid":"corr-1","causationid":"caus-1","traceparent":"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"}`

func publishableRecord(t testingT) dmpfpostgres.Claimed {
	t.Helper()

	payload, typeURL, err := envelope.Pack(&eventv1.OrderPlaced{OrderId: "o-1", ItemCount: 3})
	if err != nil {
		t.Fatalf("Pack() = %v, want nil", err)
	}
	return dmpfpostgres.Claimed{
		MessageID:        "msg-1",
		MessageType:      "com.company.orders.order-placed.v1",
		SchemaVersion:    typeURL,
		AggregateType:    "order",
		AggregateID:      "o-1",
		AggregateVersion: 7,
		PartitionKey:     "c-1",
		Destination:      "orders.integration",
		Payload:          payload,
		PayloadHash:      payloadhash.Sum(payload),
		Metadata:         []byte(publishableMetadata),
		OccurredAt:       dmpfports.Instant(1_755_432_000_000_000_000),
		AttemptCount:     1,
	}
}
