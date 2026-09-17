//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	outboxOrderID  = "o-1001"
	outboxOccurred = ports.Instant(1_755_432_000_000_000_000)
	orderPlacedV1  = "com.company.orders.order-placed.v1"
)

type orderPlaced struct {
	Order string
	Items int
}

func (orderPlaced) EventName() string { return "orders.order-placed" }

// testMapper is the mapper these tests need before Fase 5 writes the real one.
// shift changes the payload for the same event, which is how the frozen-bytes
// clause gets a second mapper that disagrees with the first.
type testMapper struct {
	shift        int32
	declaredType string
}

func (m testMapper) Map(event domain.DomainEvent) (postgres.Mapped, error) {
	placed, ok := event.(orderPlaced)
	if !ok {
		return postgres.Mapped{}, postgres.ErrUnmappedEvent
	}
	declared := m.declaredType
	if declared == "" {
		declared = orderPlacedV1
	}
	return postgres.Mapped{
		Message: &eventv1.OrderPlaced{
			OrderId:   placed.Order,
			ItemCount: int32(placed.Items) + m.shift,
		},
		Type: declared,
	}, nil
}

// strayDomainEvent has no contract in any mapper, so it is the event that
// proves an unpublishable event never reaches the table.
type strayDomainEvent struct{}

func (strayDomainEvent) EventName() string { return "test.stray" }

type outboxResources struct{ Outbox ports.Outbox }

func bindOutbox(mapper postgres.EventMapper) func(*postgres.Tx) outboxResources {
	return func(tx *postgres.Tx) outboxResources {
		return outboxResources{Outbox: tx.Outbox(mapper)}
	}
}

func outboxEntry(id ports.MessageID, event domain.DomainEvent) ports.OutboxEntry {
	return ports.OutboxEntry{
		MessageID:        id,
		OccurredAt:       outboxOccurred,
		Intent:           ports.PublishIntent{Destination: "orders.events", PartitionKey: outboxOrderID},
		AggregateType:    "orders.Order",
		AggregateID:      outboxOrderID,
		AggregateVersion: 1,
		Event:            event,
	}
}

func placedEvent(items int) orderPlaced {
	return orderPlaced{Order: outboxOrderID, Items: items}
}

func enqueued(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM dmpf_outbox").Scan(&count); err != nil {
		t.Fatalf("count(*) = %v, want nil", err)
	}
	return count
}

// enqueueOne runs one Enqueue in its own transaction, which is what every
// clause below needs before it can assert on the committed row or on its absence.
func enqueueOne(t *testing.T, pool *pgxpool.Pool, mapper postgres.EventMapper, entry ports.OutboxEntry) error {
	t.Helper()

	uow := postgres.NewUnitOfWork(pool, bindOutbox(mapper))
	return uow.Within(context.Background(), func(ctx context.Context, res outboxResources) error {
		return res.Outbox.Enqueue(ctx, entry)
	})
}

func TestEnqueueWritesTheInitialValues(t *testing.T) {
	pool := openPool(t)

	if err := enqueueOne(t, pool, testMapper{}, outboxEntry("m-000001", placedEvent(3))); err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	var (
		messageID, messageType, schemaVersion string
		aggregateType, aggregateID            string
		aggregateVersion                      int64
		partitionKey, destination             string
		payload                               []byte
		hash, metadata, status                string
		occurredAt, availableAt               int64
		attemptCount                          int
		lockedBy, lastError                   *string
		lockedUntil, publishedAt              *int64
	)
	err := pool.QueryRow(context.Background(), `
		SELECT message_id, message_type, schema_version, aggregate_type, aggregate_id,
		       aggregate_version, partition_key, destination, payload, payload_hash,
		       metadata::text, occurred_at, available_at, attempt_count, status,
		       locked_by, locked_until, published_at, last_error
		FROM dmpf_outbox`).Scan(
		&messageID, &messageType, &schemaVersion, &aggregateType, &aggregateID,
		&aggregateVersion, &partitionKey, &destination, &payload, &hash,
		&metadata, &occurredAt, &availableAt, &attemptCount, &status,
		&lockedBy, &lockedUntil, &publishedAt, &lastError)
	if err != nil {
		t.Fatalf("SELECT = %v, want nil", err)
	}

	t.Run("drain state starts idle", func(t *testing.T) {
		if status != "pending" {
			t.Errorf("status = %q, want \"pending\" (OBX-05)", status)
		}
		if availableAt != occurredAt {
			t.Errorf("available_at = %d, want occurred_at %d (OBX-05)", availableAt, occurredAt)
		}
		if attemptCount != 0 {
			t.Errorf("attempt_count = %d, want 0 (OBX-05)", attemptCount)
		}
		if lockedBy != nil || lockedUntil != nil || publishedAt != nil || lastError != nil {
			t.Errorf("lease and outcome columns are not NULL: locked_by=%v locked_until=%v published_at=%v last_error=%v (BLK-05)",
				lockedBy, lockedUntil, publishedAt, lastError)
		}
	})

	t.Run("routing and identity come from the entry", func(t *testing.T) {
		if messageID != "m-000001" {
			t.Errorf("message_id = %q, want \"m-000001\"", messageID)
		}
		if messageType != orderPlacedV1 {
			t.Errorf("message_type = %q, want %q (PTB-03)", messageType, orderPlacedV1)
		}
		if want := "type.googleapis.com/company.orders.event.v1.OrderPlaced"; schemaVersion != want {
			t.Errorf("schema_version = %q, want %q", schemaVersion, want)
		}
		if aggregateType != "orders.Order" || aggregateID != outboxOrderID || aggregateVersion != 1 {
			t.Errorf("aggregate = (%q, %q, %d), want (\"orders.Order\", %q, 1)",
				aggregateType, aggregateID, aggregateVersion, outboxOrderID)
		}
		if partitionKey != outboxOrderID || destination != "orders.events" {
			t.Errorf("routing = (%q, %q), want (%q, \"orders.events\")", partitionKey, destination, outboxOrderID)
		}
		if occurredAt != int64(outboxOccurred) {
			t.Errorf("occurred_at = %d, want %d", occurredAt, outboxOccurred)
		}
	})

	t.Run("payload is the serialized contract and metadata is empty", func(t *testing.T) {
		want, _, err := envelope.Pack(&eventv1.OrderPlaced{OrderId: outboxOrderID, ItemCount: 3})
		if err != nil {
			t.Fatalf("Pack() = %v, want nil", err)
		}
		if string(payload) != string(want) {
			t.Errorf("payload = %x, want %x (BLK-03, ADR-021)", payload, want)
		}
		if hash != payloadhash.Sum(payload) {
			t.Errorf("payload_hash = %q, want %q (ENV-18)", hash, payloadhash.Sum(payload))
		}
		if metadata != "{}" {
			t.Errorf("metadata = %q, want \"{}\" (OBX-02)", metadata)
		}
	})
}

func TestEnqueueRejectsADuplicateMessageID(t *testing.T) {
	pool := openPool(t)
	uow := postgres.NewUnitOfWork(pool, bindOutbox(testMapper{}))

	err := uow.Within(context.Background(), func(ctx context.Context, res outboxResources) error {
		if err := res.Outbox.Enqueue(ctx, outboxEntry("m-000001", placedEvent(3))); err != nil {
			return err
		}
		return res.Outbox.Enqueue(ctx, outboxEntry("m-000001", placedEvent(4)))
	})

	if !errors.Is(err, postgres.ErrDuplicateMessage) {
		t.Fatalf("Within() = %v, want ErrDuplicateMessage (OBX-01)", err)
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("errors.As(%v, *pgconn.PgError) = false, want the driver error preserved", err)
	}
	if pgErr.Code != "23505" {
		t.Errorf("SQLSTATE = %q, want \"23505\"", pgErr.Code)
	}
	if pgErr.ConstraintName != "dmpf_outbox_message_id_unique" {
		t.Errorf("constraint = %q, want \"dmpf_outbox_message_id_unique\" — the schema is what refuses", pgErr.ConstraintName)
	}
	if got := enqueued(t, pool); got != 0 {
		t.Errorf("kept %d rows after the rollback, want 0", got)
	}
}

func TestEnqueueRefusesBeforeTouchingTheTable(t *testing.T) {
	cases := []struct {
		name   string
		mapper postgres.EventMapper
		entry  ports.OutboxEntry
		want   error
	}{
		{
			name:   "empty message id",
			mapper: testMapper{},
			entry:  outboxEntry("", placedEvent(3)),
			want:   postgres.ErrEmptyMessageID,
		},
		{
			name:   "unmapped event",
			mapper: testMapper{},
			entry:  outboxEntry("m-000001", strayDomainEvent{}),
			want:   postgres.ErrUnmappedEvent,
		},
		{
			name:   "major mismatch",
			mapper: testMapper{declaredType: "com.company.orders.order-placed.v2"},
			entry:  outboxEntry("m-000001", placedEvent(3)),
			want:   envelope.ErrMajorMismatch,
		},
		{
			name:   "ARN as destination",
			mapper: testMapper{},
			entry:  withDestination(outboxEntry("m-000001", placedEvent(3)), "arn:aws:sns:us-east-1:123456789012:orders"),
			want:   postgres.ErrInvalidDestination,
		},
		{
			name:   "path as destination",
			mapper: testMapper{},
			entry:  withDestination(outboxEntry("m-000001", placedEvent(3)), "orders/events"),
			want:   postgres.ErrInvalidDestination,
		},
		{
			name:   "broker URL as destination",
			mapper: testMapper{},
			entry:  withDestination(outboxEntry("m-000001", placedEvent(3)), "kafka://orders"),
			want:   postgres.ErrInvalidDestination,
		},
		{
			name:   "uppercase destination",
			mapper: testMapper{},
			entry:  withDestination(outboxEntry("m-000001", placedEvent(3)), "Orders.Events"),
			want:   postgres.ErrInvalidDestination,
		},
		{
			name:   "empty destination",
			mapper: testMapper{},
			entry:  withDestination(outboxEntry("m-000001", placedEvent(3)), ""),
			want:   postgres.ErrInvalidDestination,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := openPool(t)

			err := enqueueOne(t, pool, tc.mapper, tc.entry)

			if !errors.Is(err, tc.want) {
				t.Fatalf("Within() = %v, want %v", err, tc.want)
			}
			if got := enqueued(t, pool); got != 0 {
				t.Fatalf("kept %d rows, want 0 — the refusal rolls back", got)
			}
		})
	}
}

func TestEnqueueAcceptsALogicalDestination(t *testing.T) {
	pool := openPool(t)
	entry := withDestination(outboxEntry("m-000001", placedEvent(3)), "billing.invoice.issued")

	if err := enqueueOne(t, pool, testMapper{}, entry); err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	if got := enqueued(t, pool); got != 1 {
		t.Fatalf("kept %d rows, want 1", got)
	}
}

// The bytes are the fact. A mapper that changes later describes new events, and
// must not rewrite what was already serialized (BLK-03, ADR-021, ENV-18).
func TestEnqueueFreezesThePayloadBytes(t *testing.T) {
	pool := openPool(t)

	if err := enqueueOne(t, pool, testMapper{}, outboxEntry("m-000001", placedEvent(3))); err != nil {
		t.Fatalf("first Within() = %v, want nil", err)
	}
	before := payloadOf(t, pool, "m-000001")

	if err := enqueueOne(t, pool, testMapper{shift: 99}, outboxEntry("m-000002", placedEvent(3))); err != nil {
		t.Fatalf("second Within() = %v, want nil", err)
	}

	after := payloadOf(t, pool, "m-000001")
	if string(after) != string(before) {
		t.Fatalf("the first payload changed: %x, want %x", after, before)
	}
	if second := payloadOf(t, pool, "m-000002"); string(second) == string(before) {
		t.Fatal("the second mapper produced identical bytes; the clause proves nothing")
	}
	if hash := hashOf(t, pool, "m-000001"); hash != payloadhash.Sum(after) {
		t.Fatalf("payload_hash = %q, want %q", hash, payloadhash.Sum(after))
	}
}

func TestEnqueueIsDeterministic(t *testing.T) {
	pool := openPool(t)

	for _, id := range []ports.MessageID{"m-000001", "m-000002"} {
		if err := enqueueOne(t, pool, testMapper{}, outboxEntry(id, placedEvent(3))); err != nil {
			t.Fatalf("Within(%s) = %v, want nil", id, err)
		}
	}

	first, second := payloadOf(t, pool, "m-000001"), payloadOf(t, pool, "m-000002")
	if string(first) != string(second) {
		t.Fatalf("the same event serialized differently: %x and %x", first, second)
	}
}

func withDestination(entry ports.OutboxEntry, destination string) ports.OutboxEntry {
	entry.Intent.Destination = destination
	return entry
}

const testTraceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

func withContext(entry ports.OutboxEntry) ports.OutboxEntry {
	entry.Context = ports.MessageContext{CorrelationID: "corr-1", CausationID: "caus-1", Traceparent: testTraceparent}
	return entry
}

func metadataOf(t *testing.T, pool *pgxpool.Pool, id ports.MessageID) map[string]string {
	t.Helper()

	var raw []byte
	if err := pool.QueryRow(context.Background(),
		"SELECT metadata FROM dmpf_outbox WHERE message_id = $1", string(id)).Scan(&raw); err != nil {
		t.Fatalf("SELECT metadata = %v, want nil", err)
	}
	metadata := map[string]string{}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("metadata %s is not a flat JSON object of strings: %v", raw, err)
	}
	return metadata
}

// The three ENV-08 attributes land in metadata exactly as the adapter authored
// them (FND-07 §8.6 item 3): the writer copies, it never invents (OBX-02).
func TestEnqueueWritesTheMessageContextAsMetadata(t *testing.T) {
	pool := openPool(t)

	if err := enqueueOne(t, pool, testMapper{}, withContext(outboxEntry("m-000001", placedEvent(3)))); err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	got := metadataOf(t, pool, "m-000001")
	want := map[string]string{"correlationid": "corr-1", "causationid": "caus-1", "traceparent": testTraceparent}
	if len(got) != len(want) {
		t.Fatalf("metadata = %v, want exactly %v", got, want)
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("metadata[%q] = %q, want %q", key, got[key], value)
		}
	}
}

func TestEnqueueWritesOnlyTheAuthoredAttributes(t *testing.T) {
	pool := openPool(t)
	entry := outboxEntry("m-000001", placedEvent(3))
	entry.Context = ports.MessageContext{CorrelationID: "corr-1"}

	if err := enqueueOne(t, pool, testMapper{}, entry); err != nil {
		t.Fatalf("Within() = %v, want nil", err)
	}

	got := metadataOf(t, pool, "m-000001")
	if len(got) != 1 || got["correlationid"] != "corr-1" {
		t.Fatalf("metadata = %v, want only {correlationid: corr-1} — an absent attribute is absent, not empty", got)
	}
}

func payloadOf(t *testing.T, pool *pgxpool.Pool, id ports.MessageID) []byte {
	t.Helper()

	var payload []byte
	if err := pool.QueryRow(context.Background(),
		"SELECT payload FROM dmpf_outbox WHERE message_id = $1", string(id)).Scan(&payload); err != nil {
		t.Fatalf("SELECT payload = %v, want nil", err)
	}
	return payload
}

func hashOf(t *testing.T, pool *pgxpool.Pool, id ports.MessageID) string {
	t.Helper()

	var hash string
	if err := pool.QueryRow(context.Background(),
		"SELECT payload_hash FROM dmpf_outbox WHERE message_id = $1", string(id)).Scan(&hash); err != nil {
		t.Fatalf("SELECT payload_hash = %v, want nil", err)
	}
	return hash
}
