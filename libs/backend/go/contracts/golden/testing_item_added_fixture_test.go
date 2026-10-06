package golden

import (
	"testing"

	"google.golang.org/protobuf/proto"

	testingv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/dmpf/testing/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const testingItemAddedPath = "libs/backend/go/contracts/fixtures/testing/v1/item-added.golden"

var testingItemAddedSpec = tb.Spec[proto.Message]{
	Path:    testingItemAddedPath,
	Source:  "urn:dmpf:orders",
	Subject: "order/o-1001",
	Identity: golden.Identity{
		Fixture:    "testing/v1/item-added",
		Contract:   golden.Contract{Package: "dmpf.testing.v1", Message: "ItemAdded"},
		Type:       "dmpf.testing.item-added.v1",
		DataSchema: "type.googleapis.com/dmpf.testing.v1.ItemAdded",
	},
	Unknown:            7,
	New:                func() proto.Message { return &testingv1.ItemAdded{} },
	FromFields:         func(fields map[string]string) (proto.Message, error) { return testingItemAdded(fields) },
	Build:              buildTestingItemAdded,
	WantCases:          3,
	WantDiscriminators: 2,
}

func testingItemAddedFields(orderID, sku, quantity string) map[string]string {
	return map[string]string{
		"order_id": orderID,
		"sku":      sku,
		"quantity": quantity,
	}
}

func testingItemAdded(fields map[string]string) (*testingv1.ItemAdded, error) {
	quantity, err := tb.ParseInt(fields, "quantity", 32)
	if err != nil {
		return nil, err
	}
	return &testingv1.ItemAdded{
		OrderId:  fields["order_id"],
		Sku:      fields["sku"],
		Quantity: int32(quantity),
	}, nil
}

func buildTestingItemAdded(t *testing.T, s tb.Spec[proto.Message]) golden.Fixture {
	t.Helper()

	allPresent := s.PackedCase(t, "all-conditionals-present",
		"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
		tb.WithConditionals(s.BaseEnvelope()), testingItemAddedFields("o-1001", "SKU-001", "2"))

	absentEnv := s.BaseEnvelope()
	absentEnv["id"] = "evt-0002"
	allAbsent := s.PackedCase(t, "all-conditionals-absent",
		"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
		absentEnv, testingItemAddedFields("o-1002", "SKU-002", "1"))

	zeroEnv := s.BaseEnvelope()
	zeroEnv["id"] = "evt-0003"
	quantityZero := s.PackedCase(t, "quantity-zero",
		"quantity no valor zero: o campo não aparece no wire, e o leitor precisa distinguir ausência de zero pelo contrato.",
		zeroEnv, testingItemAddedFields("o-1003", "SKU-003", "0"))

	unknownEnv := s.BaseEnvelope()
	unknownEnv["id"] = "evt-2001"
	unknownField := s.UnknownFieldCase(t, "unknown-field",
		"Bytes canônicos mais um campo de número 7 (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).",
		unknownEnv, testingItemAddedFields("o-2001", "SKU-001", "2"))

	nonCanonicalEnv := s.BaseEnvelope()
	nonCanonicalEnv["id"] = "evt-2002"
	nonCanonical := s.NonCanonicalCase(t, "non-canonical-field-order",
		"Campos em ordem decrescente de número: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18).",
		nonCanonicalEnv, testingItemAddedFields("o-2002", "SKU-002", "5"))

	return s.Fixture(
		[]golden.Case{allPresent, allAbsent, quantityZero},
		[]golden.Case{unknownField, nonCanonical},
	)
}
