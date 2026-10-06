//go:build integration

package postgres_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func TestQuarantinePreservesEnvelopeByteIdentical(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	raw := []byte{0x0a, 0x03, 0x61, 0x2d, 0x31, 0xff, 0x00, 0xfe}

	q := postgres.NewQuarantine(pool)
	if err := q.Quarantine(ctx, ports.Contained{
		Consumer:  "orders",
		MessageID: "m-1",
		Reason:    ports.ReasonTerminalFailure,
		Envelope:  raw,
		Error:     "some error",
		At:        100,
	}); err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}

	var stored []byte
	if err := pool.QueryRow(ctx,
		`SELECT envelope FROM quarantine WHERE consumer_name = 'orders' AND message_id = 'm-1'`).Scan(&stored); err != nil {
		t.Fatalf("SELECT envelope = %v", err)
	}
	if !bytes.Equal(stored, raw) {
		t.Fatalf("stored envelope differs from original — GAR-07 violated")
	}
}

func TestQuarantineStoresLastError(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	q := postgres.NewQuarantine(pool)
	if err := q.Quarantine(ctx, ports.Contained{
		Consumer:  "orders",
		MessageID: "m-2",
		Reason:    ports.ReasonCollision,
		Envelope:  []byte{0x01},
		Error:     "hash mismatch",
		At:        200,
	}); err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}

	var lastError *string
	if err := pool.QueryRow(ctx,
		`SELECT last_error FROM quarantine WHERE consumer_name = 'orders' AND message_id = 'm-2'`).Scan(&lastError); err != nil {
		t.Fatalf("SELECT last_error = %v", err)
	}
	if lastError == nil || *lastError != "hash mismatch" {
		t.Fatalf("last_error = %v, want 'hash mismatch'", lastError)
	}
}

func TestQuarantineNullLastErrorWhenEmpty(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	q := postgres.NewQuarantine(pool)
	if err := q.Quarantine(ctx, ports.Contained{
		Consumer:  "orders",
		MessageID: "m-3",
		Reason:    ports.ReasonInvalidEnvelope,
		Envelope:  []byte{0x01},
		At:        300,
	}); err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}

	var lastError *string
	if err := pool.QueryRow(ctx,
		`SELECT last_error FROM quarantine WHERE consumer_name = 'orders' AND message_id = 'm-3'`).Scan(&lastError); err != nil {
		t.Fatalf("SELECT last_error = %v", err)
	}
	if lastError != nil {
		t.Fatalf("last_error = %v, want NULL", *lastError)
	}
}

func TestQuarantineRejectsEmptyFields(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	q := postgres.NewQuarantine(pool)

	cases := []struct {
		name string
		c    ports.Contained
	}{
		{"empty consumer", ports.Contained{Consumer: "", MessageID: "m-1", Reason: ports.ReasonTerminalFailure, Envelope: []byte{1}, At: 100}},
		{"empty reason", ports.Contained{Consumer: "orders", MessageID: "m-1", Reason: "", Envelope: []byte{1}, At: 100}},
		{"empty envelope", ports.Contained{Consumer: "orders", MessageID: "m-1", Reason: ports.ReasonTerminalFailure, Envelope: nil, At: 100}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := q.Quarantine(ctx, tc.c)
			if !errors.Is(err, postgres.ErrInvalidContainment) {
				t.Fatalf("Quarantine() = %v, want ErrInvalidContainment", err)
			}
		})
	}
}

func quarantineRows(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM quarantine").Scan(&n); err != nil {
		t.Fatalf("count(*) = %v", err)
	}
	return n
}

func TestQuarantineContainsTheSameEnvelopeOnce(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	q := postgres.NewQuarantine(pool)
	contained := ports.Contained{Consumer: "orders", MessageID: "m-1", Reason: ports.ReasonTerminalFailure, Envelope: []byte{0x0a, 0x01}, At: 100}

	for attempt := 1; attempt <= 2; attempt++ {
		contained.At = ports.Instant(100 * attempt)
		if err := q.Quarantine(ctx, contained); err != nil {
			t.Fatalf("Quarantine() attempt %d = %v, want nil: a redelivered poison message is contained again, not refused", attempt, err)
		}
	}
	if got := quarantineRows(t, pool); got != 1 {
		t.Fatalf("quarantine rows = %d, want 1: the same envelope is one containment", got)
	}
}

func TestQuarantineKeepsDistinctEnvelopesAndConsumersApart(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	q := postgres.NewQuarantine(pool)

	for _, c := range []ports.Contained{
		{Consumer: "orders", MessageID: "m-1", Reason: ports.ReasonTerminalFailure, Envelope: []byte{0x01}, At: 100},
		{Consumer: "orders", MessageID: "m-2", Reason: ports.ReasonTerminalFailure, Envelope: []byte{0x02}, At: 100},
		{Consumer: "billing", MessageID: "m-1", Reason: ports.ReasonTerminalFailure, Envelope: []byte{0x01}, At: 100},
	} {
		if err := q.Quarantine(ctx, c); err != nil {
			t.Fatalf("Quarantine(%s, %v) = %v, want nil", c.Consumer, c.Envelope, err)
		}
	}
	if got := quarantineRows(t, pool); got != 3 {
		t.Fatalf("quarantine rows = %d, want 3", got)
	}
}

func TestMigrateConsolidatesTheQuarantineOfAnOlderSchema(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	for _, stmt := range []string{
		"DROP INDEX quarantine_consumer_name_envelope_digest_idx",
		"ALTER TABLE quarantine DROP COLUMN envelope_digest",
		`INSERT INTO quarantine (consumer_name, message_id, reason, envelope, contained_at) VALUES
		   ('orders', 'm-1', 'terminal-failure', '\x0a01', 100),
		   ('orders', 'm-1', 'terminal-failure', '\x0a01', 200),
		   ('orders', 'm-2', 'terminal-failure', '\x0a02', 300)`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seeding the older schema: %s: %v", stmt, err)
		}
	}

	if err := postgres.Migrate(ctx, pool, allCapabilities, probeSchema); err != nil {
		t.Fatalf("Migrate() over the older schema = %v, want nil", err)
	}

	if got := quarantineRows(t, pool); got != 2 {
		t.Fatalf("quarantine rows = %d, want 2: the duplicate containment collapses into the oldest", got)
	}
	var contained int64
	if err := pool.QueryRow(ctx, "SELECT contained_at FROM quarantine WHERE message_id = 'm-1'").Scan(&contained); err != nil {
		t.Fatalf("SELECT contained_at = %v", err)
	}
	if contained != 100 {
		t.Fatalf("kept contained_at %d, want 100: the first containment is the one kept", contained)
	}
	if !exists(t, ctx, pool, indexExistsQuery, "quarantine_consumer_name_envelope_digest_idx") {
		t.Fatal("the unique index was not created over the consolidated table")
	}
}

func TestMigrateLeavesTheContainmentsOfAnOlderReplicaAlone(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	if err := postgres.NewQuarantine(pool).Quarantine(ctx, ports.Contained{
		Consumer: "orders", MessageID: "m-1", Reason: ports.ReasonTerminalFailure, Envelope: []byte{0x0a, 0x01}, At: 100,
	}); err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}
	for _, at := range []int64{200, 300} {
		if _, err := pool.Exec(ctx, `INSERT INTO quarantine (consumer_name, message_id, reason, envelope, contained_at)
			VALUES ('orders', 'm-1', 'terminal-failure', '\x0a01', $1)`, at); err != nil {
			t.Fatalf("INSERT without envelope_digest = %v, want nil: a replica from before the digest keeps containing during the rollout", err)
		}
	}

	if err := postgres.Migrate(ctx, pool, allCapabilities, probeSchema); err != nil {
		t.Fatalf("Migrate() over containments without digest = %v, want nil: the backfill runs only when the column is created", err)
	}
	if got := quarantineRows(t, pool); got != 3 {
		t.Fatalf("quarantine rows = %d, want 3: a boot without schema change neither backfills nor consolidates", got)
	}
}
