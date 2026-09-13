package golden

import (
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
)

const itemAddedPath = "../../../../../contracts/fixtures/orders/event/v1/item-added.golden"

var itemAddedSpec = fixtureSpec{
	path: itemAddedPath,
	identity: identity{
		Fixture:    "orders/event/v1/item-added",
		Contract:   contract{Package: "company.orders.event.v1", Message: "ItemAdded"},
		Type:       "com.company.orders.item-added.v1",
		DataSchema: "type.googleapis.com/company.orders.event.v1.ItemAdded",
	},
	fieldNumbers:       fieldNumbers{unknown: 7},
	newMessage:         func() proto.Message { return &eventv1.ItemAdded{} },
	messageFromFields:  func(fields map[string]string) (proto.Message, error) { return itemAdded(fields) },
	build:              buildItemAdded,
	wantCases:          3,
	wantDiscriminators: 2,
}

func itemAddedFields(orderID, sku, quantity string) map[string]string {
	return map[string]string{
		"order_id": orderID,
		"sku":      sku,
		"quantity": quantity,
	}
}

func itemAdded(fields map[string]string) (*eventv1.ItemAdded, error) {
	quantity, err := parseInt(fields, "quantity", 32)
	if err != nil {
		return nil, err
	}
	return &eventv1.ItemAdded{
		OrderId:  fields["order_id"],
		Sku:      fields["sku"],
		Quantity: int32(quantity),
	}, nil
}

func buildItemAdded(t *testing.T, s fixtureSpec) fixtureDoc {
	t.Helper()

	allPresent := s.packedCase(t, "all-conditionals-present",
		"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
		withConditionals(s.baseEnvelope()), itemAddedFields("o-1001", "SKU-001", "2"))

	absentEnv := s.baseEnvelope()
	absentEnv["id"] = "evt-0002"
	allAbsent := s.packedCase(t, "all-conditionals-absent",
		"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
		absentEnv, itemAddedFields("o-1002", "SKU-002", "1"))

	zeroEnv := s.baseEnvelope()
	zeroEnv["id"] = "evt-0003"
	quantityZero := s.packedCase(t, "quantity-zero",
		"quantity no valor zero: o campo não aparece no wire, e o leitor precisa distinguir ausência de zero pelo contrato.",
		zeroEnv, itemAddedFields("o-1003", "SKU-003", "0"))

	unknownEnv := s.baseEnvelope()
	unknownEnv["id"] = "evt-2001"
	unknownField := s.unknownFieldCase(t, "unknown-field",
		"Bytes canônicos mais um campo de número 7 (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).",
		unknownEnv, itemAddedFields("o-2001", "SKU-001", "2"))

	nonCanonicalEnv := s.baseEnvelope()
	nonCanonicalEnv["id"] = "evt-2002"
	nonCanonical := s.nonCanonicalCase(t, "non-canonical-field-order",
		"Campos em ordem decrescente de número: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18).",
		nonCanonicalEnv, itemAddedFields("o-2002", "SKU-002", "5"))

	return fixtureDoc{
		FormatVersion:  formatVersion,
		Identity:       s.identity,
		Covers:         covers{ProfileMajor: "1", ContractMajor: "v1"},
		Cases:          []fixtureCase{allPresent, allAbsent, quantityZero},
		Discriminators: []fixtureCase{unknownField, nonCanonical},
	}
}
