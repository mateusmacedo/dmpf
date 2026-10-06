package golden

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

var specs = []tb.Spec[proto.Message]{reservationConfirmedSpec, reservationCancelledSpec}

func TestGolden(t *testing.T) {
	tb.GoldenSuite(t, specs...)
}

func TestUpdateGolden(t *testing.T) { tb.UpdateGolden(t, specs...) }
