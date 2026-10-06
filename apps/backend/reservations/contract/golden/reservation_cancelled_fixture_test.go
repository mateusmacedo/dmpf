package golden

import (
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/dmpf/apps/backend/reservations/contract/gen/go/company/reservations/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const reservationCancelledPath = "apps/backend/reservations/contract/fixtures/event/v1/reservation-cancelled.golden"

var reservationCancelledSpec = tb.Spec[proto.Message]{
	Path:    reservationCancelledPath,
	Source:  "urn:dmpf:orders",
	Subject: "order/o-1001",
	Identity: golden.Identity{
		Fixture:    "reservations/event/v1/reservation-cancelled",
		Contract:   golden.Contract{Package: "company.reservations.event.v1", Message: "ReservationCancelled"},
		Type:       "com.company.reservations.reservation-cancelled.v1",
		DataSchema: "type.googleapis.com/company.reservations.event.v1.ReservationCancelled",
	},
	Unknown:            7,
	New:                func() proto.Message { return &eventv1.ReservationCancelled{} },
	FromFields:         func(fields map[string]string) (proto.Message, error) { return reservationCancelled(fields), nil },
	Build:              buildReservationCancelled,
	WantCases:          2,
	WantDiscriminators: 2,
}

func reservationCancelledFields(orderID string) map[string]string {
	return map[string]string{"order_id": orderID}
}

func reservationCancelled(fields map[string]string) *eventv1.ReservationCancelled {
	return &eventv1.ReservationCancelled{OrderId: fields["order_id"]}
}

func buildReservationCancelled(t *testing.T, s tb.Spec[proto.Message]) golden.Fixture {
	t.Helper()

	allPresent := s.PackedCase(t, "all-conditionals-present",
		"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
		tb.WithConditionals(s.BaseEnvelope()), reservationCancelledFields("o-1001"))

	absentEnv := s.BaseEnvelope()
	absentEnv["id"] = "evt-0002"
	allAbsent := s.PackedCase(t, "all-conditionals-absent",
		"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
		absentEnv, reservationCancelledFields("o-1002"))

	unknownEnv := s.BaseEnvelope()
	unknownEnv["id"] = "evt-2001"
	unknownField := s.UnknownFieldCase(t, "unknown-field",
		"Bytes canônicos mais um campo de número 7 (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).",
		unknownEnv, reservationCancelledFields("o-2001"))

	nonCanonicalEnv := s.BaseEnvelope()
	nonCanonicalEnv["id"] = "evt-2002"
	nonCanonical := s.NonCanonicalCase(t, "non-canonical-field-order",
		"O único campo emitido duas vezes com o mesmo valor: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18).",
		nonCanonicalEnv, reservationCancelledFields("o-2002"))

	return s.Fixture(
		[]golden.Case{allPresent, allAbsent},
		[]golden.Case{unknownField, nonCanonical},
	)
}
