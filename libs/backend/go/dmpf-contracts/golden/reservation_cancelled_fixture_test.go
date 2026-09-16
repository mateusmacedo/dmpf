package golden

import (
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/reservations/event/v1"
)

const reservationCancelledPath = "../../../../../contracts/fixtures/reservations/event/v1/reservation-cancelled.golden"

var reservationCancelledSpec = fixtureSpec{
	path: reservationCancelledPath,
	identity: identity{
		Fixture:    "reservations/event/v1/reservation-cancelled",
		Contract:   contract{Package: "company.reservations.event.v1", Message: "ReservationCancelled"},
		Type:       "com.company.reservations.reservation-cancelled.v1",
		DataSchema: "type.googleapis.com/company.reservations.event.v1.ReservationCancelled",
	},
	fieldNumbers:       fieldNumbers{unknown: 7},
	newMessage:         func() proto.Message { return &eventv1.ReservationCancelled{} },
	messageFromFields:  func(fields map[string]string) (proto.Message, error) { return reservationCancelled(fields), nil },
	build:              buildReservationCancelled,
	wantCases:          2,
	wantDiscriminators: 2,
}

func reservationCancelledFields(orderID string) map[string]string {
	return map[string]string{"order_id": orderID}
}

func reservationCancelled(fields map[string]string) *eventv1.ReservationCancelled {
	return &eventv1.ReservationCancelled{OrderId: fields["order_id"]}
}

func buildReservationCancelled(t *testing.T, s fixtureSpec) fixtureDoc {
	t.Helper()

	allPresent := s.packedCase(t, "all-conditionals-present",
		"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
		withConditionals(s.baseEnvelope()), reservationCancelledFields("o-1001"))

	absentEnv := s.baseEnvelope()
	absentEnv["id"] = "evt-0002"
	allAbsent := s.packedCase(t, "all-conditionals-absent",
		"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
		absentEnv, reservationCancelledFields("o-1002"))

	unknownEnv := s.baseEnvelope()
	unknownEnv["id"] = "evt-2001"
	unknownField := s.unknownFieldCase(t, "unknown-field",
		"Bytes canônicos mais um campo de número 7 (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).",
		unknownEnv, reservationCancelledFields("o-2001"))

	nonCanonicalEnv := s.baseEnvelope()
	nonCanonicalEnv["id"] = "evt-2002"
	nonCanonical := s.nonCanonicalCase(t, "non-canonical-field-order",
		"O único campo emitido duas vezes com o mesmo valor: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18).",
		nonCanonicalEnv, reservationCancelledFields("o-2002"))

	return fixtureDoc{
		FormatVersion:  formatVersion,
		Identity:       s.identity,
		Covers:         covers{ProfileMajor: "1", ContractMajor: "v1"},
		Cases:          []fixtureCase{allPresent, allAbsent},
		Discriminators: []fixtureCase{unknownField, nonCanonical},
	}
}
