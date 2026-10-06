package application_test

import (
	"bytes"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

func TestTheResponsesKeepTheirStoredFormat(t *testing.T) {
	accepted := usecase.EncodeOutcome(application.ItemAcceptedCodec, usecase.Accepted(domain.ItemAccepted{Order: "P-1", Items: 3}))
	if golden := []byte{0x01, 'a', 0x03, 'P', '-', '1', 0x06}; !bytes.Equal(accepted, golden) {
		t.Errorf("item accepted = %x, want %x: an entry written by the previous release must still decode", accepted, golden)
	}
	placed := usecase.EncodeOutcome(application.PlacedCodec, usecase.Accepted(domain.PlacedResponse{Order: "P-1"}))
	if golden := []byte{0x01, 'a', 0x03, 'P', '-', '1'}; !bytes.Equal(placed, golden) {
		t.Errorf("placed = %x, want %x", placed, golden)
	}
}

func TestTheResponsesRoundTrip(t *testing.T) {
	accepted, err := usecase.DecodeOutcome(application.ItemAcceptedCodec,
		usecase.EncodeOutcome(application.ItemAcceptedCodec, usecase.Accepted(domain.ItemAccepted{Order: "P-2", Items: 2})))
	if err != nil || accepted.Response() != (domain.ItemAccepted{Order: "P-2", Items: 2}) {
		t.Errorf("item accepted = (%+v, %v), want {P-2 2}", accepted.Response(), err)
	}
	placed, err := usecase.DecodeOutcome(application.PlacedCodec,
		usecase.EncodeOutcome(application.PlacedCodec, usecase.Accepted(domain.PlacedResponse{Order: "P-3"})))
	if err != nil || placed.Response() != (domain.PlacedResponse{Order: "P-3"}) {
		t.Errorf("placed = (%+v, %v), want {P-3}", placed.Response(), err)
	}
}
