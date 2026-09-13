package golden

import (
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/reservations/event/v1"
)

const reservationConfirmedPath = "../../../../../contracts/fixtures/reservations/event/v1/reservation-confirmed.golden"

var reservationConfirmedSpec = fixtureSpec{
	path: reservationConfirmedPath,
	identity: identity{
		Fixture:    "reservations/event/v1/reservation-confirmed",
		Contract:   contract{Package: "company.reservations.event.v1", Message: "ReservationConfirmed"},
		Type:       "com.company.reservations.reservation-confirmed.v1",
		DataSchema: "type.googleapis.com/company.reservations.event.v1.ReservationConfirmed",
	},
	fieldNumbers:       fieldNumbers{unknown: 7},
	newMessage:         func() proto.Message { return &eventv1.ReservationConfirmed{} },
	messageFromFields:  func(fields map[string]string) (proto.Message, error) { return reservationConfirmed(fields) },
	build:              buildReservationConfirmed,
	wantCases:          3,
	wantDiscriminators: 2,
}

func reservationConfirmedFields(orderID, itemCount string) map[string]string {
	return map[string]string{
		"order_id":   orderID,
		"item_count": itemCount,
	}
}

func reservationConfirmed(fields map[string]string) (*eventv1.ReservationConfirmed, error) {
	count, err := parseInt(fields, "item_count", 32)
	if err != nil {
		return nil, err
	}
	return &eventv1.ReservationConfirmed{
		OrderId:   fields["order_id"],
		ItemCount: int32(count),
	}, nil
}

func buildReservationConfirmed(t *testing.T, s fixtureSpec) fixtureDoc {
	t.Helper()

	allPresent := s.packedCase(t, "all-conditionals-present",
		"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
		withConditionals(s.baseEnvelope()), reservationConfirmedFields("o-1001", "2"))

	absentEnv := s.baseEnvelope()
	absentEnv["id"] = "evt-0002"
	allAbsent := s.packedCase(t, "all-conditionals-absent",
		"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
		absentEnv, reservationConfirmedFields("o-1002", "1"))

	zeroEnv := s.baseEnvelope()
	zeroEnv["id"] = "evt-0003"
	itemCountZero := s.packedCase(t, "item-count-zero",
		"item_count no valor zero: o campo não aparece no wire, e o leitor precisa distinguir ausência de zero pelo contrato.",
		zeroEnv, reservationConfirmedFields("o-1003", "0"))

	unknownEnv := s.baseEnvelope()
	unknownEnv["id"] = "evt-2001"
	unknownField := s.unknownFieldCase(t, "unknown-field",
		"Bytes canônicos mais um campo de número 7 (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).",
		unknownEnv, reservationConfirmedFields("o-2001", "2"))

	nonCanonicalEnv := s.baseEnvelope()
	nonCanonicalEnv["id"] = "evt-2002"
	nonCanonical := s.nonCanonicalCase(t, "non-canonical-field-order",
		"Campos em ordem decrescente de número: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18).",
		nonCanonicalEnv, reservationConfirmedFields("o-2002", "5"))

	return fixtureDoc{
		FormatVersion:  formatVersion,
		Identity:       s.identity,
		Covers:         covers{ProfileMajor: "1", ContractMajor: "v1"},
		Cases:          []fixtureCase{allPresent, allAbsent, itemCountZero},
		Discriminators: []fixtureCase{unknownField, nonCanonical},
	}
}
