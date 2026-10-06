package golden

import (
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/dmpf/apps/backend/reservations/contract/gen/go/company/reservations/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const reservationConfirmedPath = "apps/backend/reservations/contract/fixtures/event/v1/reservation-confirmed.golden"

var reservationConfirmedSpec = tb.Spec[proto.Message]{
	Path:    reservationConfirmedPath,
	Source:  "urn:dmpf:orders",
	Subject: "order/o-1001",
	Identity: golden.Identity{
		Fixture:    "reservations/event/v1/reservation-confirmed",
		Contract:   golden.Contract{Package: "company.reservations.event.v1", Message: "ReservationConfirmed"},
		Type:       "com.company.reservations.reservation-confirmed.v1",
		DataSchema: "type.googleapis.com/company.reservations.event.v1.ReservationConfirmed",
	},
	Unknown:            7,
	New:                func() proto.Message { return &eventv1.ReservationConfirmed{} },
	FromFields:         func(fields map[string]string) (proto.Message, error) { return reservationConfirmed(fields) },
	Build:              buildReservationConfirmed,
	WantCases:          3,
	WantDiscriminators: 2,
}

func reservationConfirmedFields(orderID, itemCount string) map[string]string {
	return map[string]string{
		"order_id":   orderID,
		"item_count": itemCount,
	}
}

func reservationConfirmed(fields map[string]string) (*eventv1.ReservationConfirmed, error) {
	count, err := tb.ParseInt(fields, "item_count", 32)
	if err != nil {
		return nil, err
	}
	return &eventv1.ReservationConfirmed{
		OrderId:   fields["order_id"],
		ItemCount: int32(count),
	}, nil
}

func buildReservationConfirmed(t *testing.T, s tb.Spec[proto.Message]) golden.Fixture {
	t.Helper()

	allPresent := s.PackedCase(t, "all-conditionals-present",
		"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
		tb.WithConditionals(s.BaseEnvelope()), reservationConfirmedFields("o-1001", "2"))

	absentEnv := s.BaseEnvelope()
	absentEnv["id"] = "evt-0002"
	allAbsent := s.PackedCase(t, "all-conditionals-absent",
		"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
		absentEnv, reservationConfirmedFields("o-1002", "1"))

	zeroEnv := s.BaseEnvelope()
	zeroEnv["id"] = "evt-0003"
	itemCountZero := s.PackedCase(t, "item-count-zero",
		"item_count no valor zero: o campo não aparece no wire, e o leitor precisa distinguir ausência de zero pelo contrato.",
		zeroEnv, reservationConfirmedFields("o-1003", "0"))

	unknownEnv := s.BaseEnvelope()
	unknownEnv["id"] = "evt-2001"
	unknownField := s.UnknownFieldCase(t, "unknown-field",
		"Bytes canônicos mais um campo de número 7 (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).",
		unknownEnv, reservationConfirmedFields("o-2001", "2"))

	nonCanonicalEnv := s.BaseEnvelope()
	nonCanonicalEnv["id"] = "evt-2002"
	nonCanonical := s.NonCanonicalCase(t, "non-canonical-field-order",
		"Campos em ordem decrescente de número: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18).",
		nonCanonicalEnv, reservationConfirmedFields("o-2002", "5"))

	return s.Fixture(
		[]golden.Case{allPresent, allAbsent, itemCountZero},
		[]golden.Case{unknownField, nonCanonical},
	)
}
