package golden

import (
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const itemAddedPath = "apps/backend/orders/contract/fixtures/event/v1/item-added.golden"

var itemAddedSpec = tb.Spec[proto.Message]{
	Path:    itemAddedPath,
	Source:  "urn:dmpf:orders",
	Subject: "order/o-1001",
	Identity: golden.Identity{
		Fixture:    "orders/event/v1/item-added",
		Contract:   golden.Contract{Package: "company.orders.event.v1", Message: "ItemAdded"},
		Type:       "com.company.orders.item-added.v1",
		DataSchema: "type.googleapis.com/company.orders.event.v1.ItemAdded",
	},
	Unknown:            7,
	New:                func() proto.Message { return &eventv1.ItemAdded{} },
	FromFields:         func(fields map[string]string) (proto.Message, error) { return itemAdded(fields) },
	Build:              buildItemAdded,
	WantCases:          3,
	WantDiscriminators: 2,
}

func itemAddedFields(orderID, sku, quantity string) map[string]string {
	return map[string]string{
		"order_id": orderID,
		"sku":      sku,
		"quantity": quantity,
	}
}

func itemAdded(fields map[string]string) (*eventv1.ItemAdded, error) {
	quantity, err := tb.ParseInt(fields, "quantity", 32)
	if err != nil {
		return nil, err
	}
	return &eventv1.ItemAdded{
		OrderId:  fields["order_id"],
		Sku:      fields["sku"],
		Quantity: int32(quantity),
	}, nil
}

func buildItemAdded(t *testing.T, s tb.Spec[proto.Message]) golden.Fixture {
	t.Helper()

	allPresent := s.PackedCase(t, "all-conditionals-present",
		"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
		tb.WithConditionals(s.BaseEnvelope()), itemAddedFields("o-1001", "SKU-001", "2"))

	absentEnv := s.BaseEnvelope()
	absentEnv["id"] = "evt-0002"
	allAbsent := s.PackedCase(t, "all-conditionals-absent",
		"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
		absentEnv, itemAddedFields("o-1002", "SKU-002", "1"))

	zeroEnv := s.BaseEnvelope()
	zeroEnv["id"] = "evt-0003"
	quantityZero := s.PackedCase(t, "quantity-zero",
		"quantity no valor zero: o campo não aparece no wire, e o leitor precisa distinguir ausência de zero pelo contrato.",
		zeroEnv, itemAddedFields("o-1003", "SKU-003", "0"))

	unknownEnv := s.BaseEnvelope()
	unknownEnv["id"] = "evt-2001"
	unknownField := s.UnknownFieldCase(t, "unknown-field",
		"Bytes canônicos mais um campo de número 7 (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).",
		unknownEnv, itemAddedFields("o-2001", "SKU-001", "2"))

	nonCanonicalEnv := s.BaseEnvelope()
	nonCanonicalEnv["id"] = "evt-2002"
	nonCanonical := s.NonCanonicalCase(t, "non-canonical-field-order",
		"Campos em ordem decrescente de número: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18).",
		nonCanonicalEnv, itemAddedFields("o-2002", "SKU-002", "5"))

	return s.Fixture(
		[]golden.Case{allPresent, allAbsent, quantityZero},
		[]golden.Case{unknownField, nonCanonical},
	)
}
