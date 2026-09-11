//go:build integration

package dmpfpostgres_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

func TestQuarantinePreservesEnvelopeByteIdentical(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	raw := []byte{0x0a, 0x03, 0x61, 0x2d, 0x31, 0xff, 0x00, 0xfe}

	q := dmpfpostgres.NewQuarantine(pool)
	if err := q.Quarantine(ctx, dmpfports.Contained{
		Consumer:  "orders",
		MessageID: "m-1",
		Reason:    dmpfports.ReasonTerminalFailure,
		Envelope:  raw,
		Error:     "some error",
		At:        100,
	}); err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}

	var stored []byte
	if err := pool.QueryRow(ctx,
		`SELECT envelope FROM dmpf_quarantine WHERE consumer_name = 'orders' AND message_id = 'm-1'`).Scan(&stored); err != nil {
		t.Fatalf("SELECT envelope = %v", err)
	}
	if !bytes.Equal(stored, raw) {
		t.Fatalf("stored envelope differs from original — GAR-07 violated")
	}
}

func TestQuarantineStoresLastError(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	q := dmpfpostgres.NewQuarantine(pool)
	if err := q.Quarantine(ctx, dmpfports.Contained{
		Consumer:  "orders",
		MessageID: "m-2",
		Reason:    dmpfports.ReasonCollision,
		Envelope:  []byte{0x01},
		Error:     "hash mismatch",
		At:        200,
	}); err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}

	var lastError *string
	if err := pool.QueryRow(ctx,
		`SELECT last_error FROM dmpf_quarantine WHERE consumer_name = 'orders' AND message_id = 'm-2'`).Scan(&lastError); err != nil {
		t.Fatalf("SELECT last_error = %v", err)
	}
	if lastError == nil || *lastError != "hash mismatch" {
		t.Fatalf("last_error = %v, want 'hash mismatch'", lastError)
	}
}

func TestQuarantineNullLastErrorWhenEmpty(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	q := dmpfpostgres.NewQuarantine(pool)
	if err := q.Quarantine(ctx, dmpfports.Contained{
		Consumer:  "orders",
		MessageID: "m-3",
		Reason:    dmpfports.ReasonInvalidEnvelope,
		Envelope:  []byte{0x01},
		At:        300,
	}); err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}

	var lastError *string
	if err := pool.QueryRow(ctx,
		`SELECT last_error FROM dmpf_quarantine WHERE consumer_name = 'orders' AND message_id = 'm-3'`).Scan(&lastError); err != nil {
		t.Fatalf("SELECT last_error = %v", err)
	}
	if lastError != nil {
		t.Fatalf("last_error = %v, want NULL", *lastError)
	}
}

func TestQuarantineRejectsEmptyFields(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	q := dmpfpostgres.NewQuarantine(pool)

	cases := []struct {
		name string
		c    dmpfports.Contained
	}{
		{"empty consumer", dmpfports.Contained{Consumer: "", MessageID: "m-1", Reason: dmpfports.ReasonTerminalFailure, Envelope: []byte{1}, At: 100}},
		{"empty reason", dmpfports.Contained{Consumer: "orders", MessageID: "m-1", Reason: "", Envelope: []byte{1}, At: 100}},
		{"empty envelope", dmpfports.Contained{Consumer: "orders", MessageID: "m-1", Reason: dmpfports.ReasonTerminalFailure, Envelope: nil, At: 100}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := q.Quarantine(ctx, tc.c)
			if !errors.Is(err, dmpfpostgres.ErrInvalidContainment) {
				t.Fatalf("Quarantine() = %v, want ErrInvalidContainment", err)
			}
		})
	}
}
