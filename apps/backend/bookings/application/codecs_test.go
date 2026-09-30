package application_test

import (
	"bytes"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

func TestTheResponsesKeepTheirStoredFormat(t *testing.T) {
	for name, c := range map[string]struct {
		stored []byte
		golden []byte
	}{
		"reserved": {
			usecase.EncodeOutcome(application.ReservedCodec, usecase.Accepted(domain.ReservedResponse{BookingID: "B-100"})),
			[]byte{0x01, 'a', 0x05, 'B', '-', '1', '0', '0'},
		},
		"cancelled": {
			usecase.EncodeOutcome(application.CancelledCodec, usecase.Accepted(domain.CancelledResponse{BookingID: "B-100"})),
			[]byte{0x01, 'a', 0x05, 'B', '-', '1', '0', '0'},
		},
		"registered": {
			usecase.EncodeOutcome(application.RegisteredCodec, usecase.Accepted(domain.RegisteredResponse{Code: "room-101"})),
			[]byte{0x01, 'a', 0x08, 'r', 'o', 'o', 'm', '-', '1', '0', '1'},
		},
	} {
		if !bytes.Equal(c.stored, c.golden) {
			t.Errorf("%s: stored = %x, want %x: an entry written by the previous release must still decode", name, c.stored, c.golden)
		}
	}
}

func TestTheResponsesRoundTrip(t *testing.T) {
	reserved, err := usecase.DecodeOutcome(application.ReservedCodec, usecase.EncodeOutcome(application.ReservedCodec, usecase.Accepted(domain.ReservedResponse{BookingID: "B-1"})))
	if err != nil || reserved.Response() != (domain.ReservedResponse{BookingID: "B-1"}) {
		t.Errorf("reserved = (%+v, %v), want {B-1}", reserved.Response(), err)
	}
	cancelled, err := usecase.DecodeOutcome(application.CancelledCodec, usecase.EncodeOutcome(application.CancelledCodec, usecase.Accepted(domain.CancelledResponse{BookingID: "B-2"})))
	if err != nil || cancelled.Response() != (domain.CancelledResponse{BookingID: "B-2"}) {
		t.Errorf("cancelled = (%+v, %v), want {B-2}", cancelled.Response(), err)
	}
	registered, err := usecase.DecodeOutcome(application.RegisteredCodec, usecase.EncodeOutcome(application.RegisteredCodec, usecase.Accepted(domain.RegisteredResponse{Code: "room-9"})))
	if err != nil || registered.Response() != (domain.RegisteredResponse{Code: "room-9"}) {
		t.Errorf("registered = (%+v, %v), want {room-9}", registered.Response(), err)
	}
}
