package golden

import (
	"fmt"
	"strconv"
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
)

// The fixture lives in the contracts tree (FIX-10), five directories above this package.
const orderPlacedPath = "../../../../../contracts/fixtures/orders/event/v1/order-placed.golden"

var orderPlacedSpec = fixtureSpec{
	path: orderPlacedPath,
	identity: identity{
		Fixture:    "orders/event/v1/order-placed",
		Contract:   contract{Package: "company.orders.event.v1", Message: "OrderPlaced"},
		Type:       "com.company.orders.order-placed.v1",
		DataSchema: "type.googleapis.com/company.orders.event.v1.OrderPlaced",
	},
	fieldNumbers: fieldNumbers{unknown: 7},
	enum: enumDiscriminator{
		field:  "channel",
		values: []string{"ORDER_CHANNEL_UNSPECIFIED", "ORDER_CHANNEL_WEB", "99"},
	},
	newMessage:         func() proto.Message { return &eventv1.OrderPlaced{} },
	messageFromFields:  func(fields map[string]string) (proto.Message, error) { return orderPlaced(fields) },
	build:              buildOrderPlaced,
	wantCases:          6,
	wantDiscriminators: 3,
}

func orderPlacedFields(orderID, customerID, totalCents, channel string) map[string]string {
	return map[string]string{
		"order_id":    orderID,
		"customer_id": customerID,
		"total_cents": totalCents,
		"channel":     channel,
	}
}

// orderPlaced is the single reader of the string-typed payload; the generator
// and the oracles share it so the two never disagree on parsing.
func orderPlaced(fields map[string]string) (*eventv1.OrderPlaced, error) {
	total, err := parseInt(fields, "total_cents", 64)
	if err != nil {
		return nil, err
	}
	channel, err := channelFromString(fields["channel"])
	if err != nil {
		return nil, err
	}
	msg := &eventv1.OrderPlaced{
		OrderId:    fields["order_id"],
		CustomerId: fields["customer_id"],
		TotalCents: total,
		Channel:    channel,
	}
	if _, ok := fields["item_count"]; ok {
		count, err := parseInt(fields, "item_count", 32)
		if err != nil {
			return nil, err
		}
		msg.ItemCount = int32(count)
	}
	return msg, nil
}

func channelFromString(s string) (eventv1.OrderChannel, error) {
	if v, ok := eventv1.OrderChannel_value[s]; ok {
		return eventv1.OrderChannel(v), nil
	}
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("channel %q is neither an enum name nor a number", s)
	}
	return eventv1.OrderChannel(n), nil
}

func buildOrderPlaced(t *testing.T, s fixtureSpec) fixtureDoc {
	t.Helper()

	allPresent := s.packedCase(t, "all-conditionals-present",
		"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
		withConditionals(s.baseEnvelope()), orderPlacedFields("o-1001", "c-42", "1999", "ORDER_CHANNEL_WEB"))

	absentEnv := s.baseEnvelope()
	absentEnv["id"] = "evt-0002"
	absentEnv["causationid"] = "evt-0001"
	allAbsent := s.packedCase(t, "all-conditionals-absent",
		"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
		absentEnv, orderPlacedFields("o-1002", "c-42", "250", "ORDER_CHANNEL_APP"))

	unspecEnv := s.baseEnvelope()
	unspecEnv["id"] = "evt-0003"
	unspecified := s.packedCase(t, "channel-unspecified",
		"Enum no valor zero (ORDER_CHANNEL_UNSPECIFIED, PTB-09): o campo não aparece no wire.",
		unspecEnv, orderPlacedFields("o-1003", "c-7", "0", "ORDER_CHANNEL_UNSPECIFIED"))

	bigEnv := withConditionals(s.baseEnvelope())
	bigEnv["id"] = "evt-0004"
	bigEnv["aggregateversion"] = "2147483647"
	beyondDouble := s.packedCase(t, "total-cents-beyond-double",
		"total_cents = 2^53 + 1: um leitor que passar por double perde o último dígito (FIX-07).",
		bigEnv, orderPlacedFields("o-1004", "c-42", "9007199254740993", "ORDER_CHANNEL_WEB"))

	nanosEnv := s.baseEnvelope()
	nanosEnv["id"] = "evt-0005"
	nanosEnv["time"] = "2026-09-02T12:00:00.123456789Z"
	nanosEnv["tracestate"] = "vendor=1,other=2"
	withNanos := s.packedCase(t, "time-with-nanos",
		"Instante do fato com nanossegundos não nulos; tracestate presente sem os outros condicionais.",
		nanosEnv, orderPlacedFields("o-1005", "c-9", "12345", "ORDER_CHANNEL_APP"))

	itemCountEnv := s.baseEnvelope()
	itemCountEnv["id"] = "evt-0006"
	itemCountFields := orderPlacedFields("o-1006", "c-42", "4500", "ORDER_CHANNEL_WEB")
	itemCountFields["item_count"] = "3"
	itemCountPresent := s.packedCase(t, "item-count-present",
		"Campo aditivo item_count no número 6, acrescentado depois do 5 reservado (PTB-06): os casos anteriores, que o omitem, mantêm os mesmos bytes.",
		itemCountEnv, itemCountFields)

	unknownEnv := s.baseEnvelope()
	unknownEnv["id"] = "evt-2001"
	unknownField := s.unknownFieldCase(t, "unknown-field",
		"Bytes canônicos mais um campo de número 7 (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).",
		unknownEnv, orderPlacedFields("o-2001", "c-42", "1999", "ORDER_CHANNEL_WEB"))

	unknownEnumEnv := s.baseEnvelope()
	unknownEnumEnv["id"] = "evt-2002"
	unknownEnum := s.packedCase(t, "enum-unknown-value",
		"channel = 99, valor que o enum não declara: decodifica sem erro e é preservado numericamente.",
		unknownEnumEnv, orderPlacedFields("o-2002", "c-42", "1999", "99"))

	nonCanonicalEnv := s.baseEnvelope()
	nonCanonicalEnv["id"] = "evt-2003"
	nonCanonical := s.nonCanonicalCase(t, "non-canonical-field-order",
		"Campos em ordem decrescente de número: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18).",
		nonCanonicalEnv, orderPlacedFields("o-2003", "c-42", "1999", "ORDER_CHANNEL_WEB"))

	return fixtureDoc{
		FormatVersion:  formatVersion,
		Identity:       s.identity,
		Covers:         covers{ProfileMajor: "1", ContractMajor: "v1"},
		Cases:          []fixtureCase{allPresent, allAbsent, unspecified, beyondDouble, withNanos, itemCountPresent},
		Discriminators: []fixtureCase{unknownField, unknownEnum, nonCanonical},
	}
}
