package app

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestRequestFieldsCarryTheCorrelationOfTheMessageInFlight(t *testing.T) {
	ctx := ports.WithMessageContext(context.Background(), ports.MessageContext{CorrelationID: "corr-1"})

	fields := requestFields(ctx)

	if got := fields[logging.KeyCorrelationID]; got != "corr-1" {
		t.Fatalf("correlation = %v, want %q", got, "corr-1")
	}
}

func TestRequestFieldsOmitTheCorrelationOutsideAMessage(t *testing.T) {
	fields := requestFields(context.Background())

	if _, ok := fields[logging.KeyCorrelationID]; ok {
		t.Fatal("correlation is present without a message context; the field would carry an empty value")
	}
}
