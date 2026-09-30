package application_test

import (
	"bytes"
	"errors"
	"math"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type itemAccepted struct {
	Order string
	Items int64
}

var itemAcceptedCodec = usecase.OutcomeCodec[itemAccepted]{
	Encode: func(e *usecase.Encoder, r itemAccepted) {
		e.String(r.Order)
		e.Int(r.Items)
	},
	Decode: func(d *usecase.Decoder) itemAccepted {
		return itemAccepted{Order: d.String(), Items: d.Int()}
	},
}

func TestAnAcceptedOutcomeRoundTrips(t *testing.T) {
	stored := usecase.EncodeOutcome(itemAcceptedCodec, usecase.Accepted(itemAccepted{Order: "o-1", Items: 2}))

	back, err := usecase.DecodeOutcome(itemAcceptedCodec, stored)
	if err != nil {
		t.Fatalf("DecodeOutcome() = %v, want nil", err)
	}
	if _, rejected := back.Rejection(); rejected {
		t.Fatal("an accepted outcome came back rejected")
	}
	if got := back.Response(); got != (itemAccepted{Order: "o-1", Items: 2}) {
		t.Fatalf("Response() = %+v, want {o-1 2}", got)
	}
}

func TestARejectedOutcomeRoundTripsWithItsDetails(t *testing.T) {
	refusal := kernel.Reject("orders/item-limit-exceeded", "the order is full", kernel.Detail{Key: "limit", Value: "10"})
	stored := usecase.EncodeOutcome(itemAcceptedCodec, usecase.Rejected[itemAccepted](refusal))

	back, err := usecase.DecodeOutcome(itemAcceptedCodec, stored)
	if err != nil {
		t.Fatalf("DecodeOutcome() = %v, want nil", err)
	}
	got, rejected := back.Rejection()
	if !rejected {
		t.Fatal("a rejected outcome came back accepted: the replay would turn a 422 into a 2xx")
	}
	if got.Code() != refusal.Code() || got.Message() != refusal.Message() {
		t.Fatalf("rejection = (%s, %q), want (%s, %q)", got.Code(), got.Message(), refusal.Code(), refusal.Message())
	}
	if details := got.Details(); len(details) != 1 || details[0] != (kernel.Detail{Key: "limit", Value: "10"}) {
		t.Fatalf("details = %+v, want [{limit 10}]", details)
	}
}

func TestTheStoredOutcomeKeepsItsWireFormat(t *testing.T) {
	stored := usecase.EncodeOutcome(itemAcceptedCodec, usecase.Accepted(itemAccepted{Order: "o-1", Items: 2}))

	golden := []byte{0x01, 'a', 0x03, 'o', '-', '1', 0x04}
	if !bytes.Equal(stored, golden) {
		t.Fatalf("stored = %x, want %x: a record written by the previous release must still decode", stored, golden)
	}
}

var countCodec = usecase.OutcomeCodec[int64]{
	Encode: func(e *usecase.Encoder, r int64) { e.Int(r) },
	Decode: func(d *usecase.Decoder) int64 { return d.Int() },
}

func TestIntegersKeepTheVarintLayoutOfTheStandardLibrary(t *testing.T) {
	for value, golden := range map[int64][]byte{
		0:             {0x00},
		-1:            {0x01},
		1:             {0x02},
		-64:           {0x7f},
		64:            {0x80, 0x01},
		150:           {0xac, 0x02},
		math.MaxInt64: {0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01},
		math.MinInt64: {0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01},
	} {
		stored := usecase.EncodeOutcome(countCodec, usecase.Accepted(value))
		if want := append([]byte{0x01, 'a'}, golden...); !bytes.Equal(stored, want) {
			t.Errorf("EncodeOutcome(%d) = %x, want %x", value, stored, want)
			continue
		}
		back, err := usecase.DecodeOutcome(countCodec, stored)
		if err != nil || back.Response() != value {
			t.Errorf("DecodeOutcome(%x) = (%d, %v), want (%d, nil)", stored, back.Response(), err, value)
		}
	}
}

func TestAnUnreadableOutcomeIsReportedNeverGuessed(t *testing.T) {
	valid := usecase.EncodeOutcome(itemAcceptedCodec, usecase.Accepted(itemAccepted{Order: "o-1", Items: 2}))

	for name, stored := range map[string][]byte{
		"empty":                nil,
		"unknown version":      append([]byte{0x02}, valid[1:]...),
		"unknown branch":       append([]byte{0x01, 'x'}, valid[2:]...),
		"truncated":            valid[:len(valid)-2],
		"trailing bytes":       append(append([]byte(nil), valid...), 0x00),
		"unterminated integer": append(append([]byte(nil), valid[:len(valid)-1]...), 0x80),
		"integer overflow":     append(append([]byte(nil), valid[:len(valid)-1]...), 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x02),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := usecase.DecodeOutcome(itemAcceptedCodec, stored); !errors.Is(err, usecase.ErrOutcomeUnreadable) {
				t.Fatalf("DecodeOutcome(%x) = %v, want ErrOutcomeUnreadable", stored, err)
			}
		})
	}
}
