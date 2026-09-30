package application_test

import (
	"bytes"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

func TestTheResponsesKeepTheirStoredFormat(t *testing.T) {
	reserved := usecase.EncodeOutcome(application.ReservedCodec, usecase.Accepted(domain.ReservedResponse{Order: "P-1", Items: 3}))
	if golden := []byte{0x01, 'a', 0x03, 'P', '-', '1', 0x06}; !bytes.Equal(reserved, golden) {
		t.Errorf("reserved = %x, want %x: an entry written by the previous release must still decode", reserved, golden)
	}
	cancelled := usecase.EncodeOutcome(application.CancelledCodec, usecase.Accepted(domain.CancelledResponse{Order: "P-1"}))
	if golden := []byte{0x01, 'a', 0x03, 'P', '-', '1'}; !bytes.Equal(cancelled, golden) {
		t.Errorf("cancelled = %x, want %x", cancelled, golden)
	}
}

func TestTheResponsesRoundTrip(t *testing.T) {
	reserved, err := usecase.DecodeOutcome(application.ReservedCodec,
		usecase.EncodeOutcome(application.ReservedCodec, usecase.Accepted(domain.ReservedResponse{Order: "P-2", Items: 2})))
	if err != nil || reserved.Response() != (domain.ReservedResponse{Order: "P-2", Items: 2}) {
		t.Errorf("reserved = (%+v, %v), want {P-2 2}", reserved.Response(), err)
	}
	cancelled, err := usecase.DecodeOutcome(application.CancelledCodec,
		usecase.EncodeOutcome(application.CancelledCodec, usecase.Accepted(domain.CancelledResponse{Order: "P-3"})))
	if err != nil || cancelled.Response() != (domain.CancelledResponse{Order: "P-3"}) {
		t.Errorf("cancelled = (%+v, %v), want {P-3}", cancelled.Response(), err)
	}
}
