package golden

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

var specs = []tb.Spec[proto.Message]{bookingReservedSpec, bookingCancelledSpec, resourceRegisteredSpec}

func TestGolden(t *testing.T) {
	tb.GoldenSuite(t, func(t testing.TB, name string, r golden.Report) {
		evidence.RecordReport(t, "golden", name, r)
	}, specs...)
}

func TestUpdateGolden(t *testing.T) { tb.UpdateGolden(t, specs...) }

func leanFixture(present, absent, unknown, nonCanonical map[string]string) func(*testing.T, tb.Spec[proto.Message]) golden.Fixture {
	return func(t *testing.T, s tb.Spec[proto.Message]) golden.Fixture {
		t.Helper()

		allPresent := s.PackedCase(t, "all-conditionals-present",
			"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
			tb.WithConditionals(s.BaseEnvelope()), present)

		absentEnv := s.BaseEnvelope()
		absentEnv["id"] = "evt-0002"
		allAbsent := s.PackedCase(t, "all-conditionals-absent",
			"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
			absentEnv, absent)

		unknownEnv := s.BaseEnvelope()
		unknownEnv["id"] = "evt-2001"
		unknownField := s.UnknownFieldCase(t, "unknown-field",
			"Bytes canônicos mais um campo de número 7 (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).",
			unknownEnv, unknown)

		nonCanonicalEnv := s.BaseEnvelope()
		nonCanonicalEnv["id"] = "evt-2002"
		nonCanonicalCase := s.NonCanonicalCase(t, "non-canonical-field-order",
			"Campos em ordem decrescente de número, o instante como sub-mensagem: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18).",
			nonCanonicalEnv, nonCanonical)

		return s.Fixture(
			[]golden.Case{allPresent, allAbsent},
			[]golden.Case{unknownField, nonCanonicalCase},
		)
	}
}
