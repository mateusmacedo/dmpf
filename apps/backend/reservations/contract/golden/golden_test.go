package golden

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

var specs = []tb.Spec[proto.Message]{reservationConfirmedSpec, reservationCancelledSpec}

func TestGolden(t *testing.T) {
	tb.GoldenSuite(t, func(t testing.TB, name string, r golden.Report) {
		evidence.RecordReport(t, "golden", name, r)
	}, specs...)
}

func TestUpdateGolden(t *testing.T) { tb.UpdateGolden(t, specs...) }
