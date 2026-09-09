//go:build integration

package dmpfreference_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	dmpfreference "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/apps/backend/dmpf-reference"
	ordersapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/orders"
	dmpfkafka "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-kafka"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/appkit"
)

// e2eItems is the ItemCount of the OrderPlaced event: the number of lines of
// the order, not the quantity of the single line the test adds.
const (
	e2eOrder                 = "o-e2e"
	e2eItems                 = 1
	orderPlacedType          = "com.company.orders.order-placed.v1"
	itemAddedType            = "com.company.orders.item-added.v1"
	reservationConfirmedType = "com.company.reservations.reservation-confirmed.v1"
	publishedByFlow          = 2 // one ItemAdded and one OrderPlaced
	publishedInTotal         = 3 // plus the redelivery the test injects
)

// The three roles in one process over the real infrastructure: a write that
// leaves the business state and the outbox row in one transaction, a relay
// that drains that row into Kafka, a consumer that reserves through the inbox,
// and a redelivery of the very same message that changes nothing. At-least-once
// with an idempotent consumer is what this proves; no artifact claims exactly-once.
func TestEndToEndWriteDrainConsumeAndRedeliver(t *testing.T) {
	h := newHarness(t)
	h.startRole(t, dmpfreference.RoleRelay)
	h.startRole(t, dmpfreference.RoleConsumer)
	server := h.apiServer(t)

	t.Log("1. write: item added and order placed through the api")
	if status, body := h.call(t, http.MethodPost, server.URL+"/orders/"+e2eOrder+"/items", `{"sku":"A","quantity":2}`); status != http.StatusCreated {
		t.Fatalf("POST items: status = %d, body %s", status, body)
	}
	if status, body := h.call(t, http.MethodPost, server.URL+"/orders/"+e2eOrder+"/place", ""); status != http.StatusOK {
		t.Fatalf("POST place: status = %d, body %s", status, body)
	}

	placed, ok := h.outboxRow(t, orderPlacedType)
	if !ok {
		t.Fatal("no OrderPlaced row in the outbox: the write did not enqueue in the same transaction")
	}
	if _, ok := h.outboxRow(t, itemAddedType); !ok {
		t.Fatal("no ItemAdded row in the outbox")
	}
	for _, key := range []string{"correlationid", "causationid", "traceparent"} {
		if placed.Metadata[key] == "" {
			t.Fatalf("outbox metadata lacks %s: %v — the relay would never drain this row", key, placed.Metadata)
		}
	}
	if placed.Metadata["causationid"] != placed.MessageID {
		t.Fatalf("causationid = %q, want the message's own id %q (the chain starts at the edge)", placed.Metadata["causationid"], placed.MessageID)
	}

	t.Log("2. drain: the relay marks the row published")
	h.waitUntil(t, "the OrderPlaced row is published", func() bool {
		row, ok := h.outboxRow(t, orderPlacedType)
		return ok && row.Status == "published"
	})

	t.Log("3. consume: one reservation and one inbox row, hash intact")
	h.waitUntil(t, "the reservation exists", func() bool { return h.count(t, "dmpf_example_reservations") == 1 })
	h.waitUntil(t, "the group consumed the flow's records", func() bool { return h.consumedEverything(t, publishedByFlow) })

	status, hash, ok := h.inboxRow(t, placed.MessageID)
	if !ok || status != "processed" {
		t.Fatalf("inbox row of %s = (%q, %v), want processed", placed.MessageID, status, ok)
	}
	if hash != placed.PayloadHash {
		t.Fatalf("inbox payload_hash = %s, want the outbox's %s — the bytes did not travel intact", hash, placed.PayloadHash)
	}
	if n := h.count(t, "dmpf_inbox"); n != 1 {
		t.Fatalf("inbox rows = %d, want 1: the ItemAdded delivery is acknowledged by the sink, never registered", n)
	}
	if n := h.count(t, "dmpf_quarantine"); n != 0 {
		t.Fatalf("quarantine rows = %d, want 0", n)
	}

	t.Log("4. redeliver: the same message once more")
	cfg := h.config(t, dmpfreference.RoleRelay)
	catalog, err := dmpfreference.NewCatalog(cfg)
	if err != nil {
		t.Fatalf("NewCatalog() = %v", err)
	}
	publisher, err := dmpfkafka.NewPublisher(dmpfreference.NewKafkaConfig(context.Background(), cfg, h.rt, catalog), nil)
	if err != nil {
		t.Fatalf("NewPublisher() = %v", err)
	}
	defer publisher.Close()
	if err := publisher.Publish(context.Background(), ordersapp.Destination, appkit.RawOrderPlaced(t, placed.MessageID, e2eOrder, e2eItems)); err != nil {
		t.Fatalf("Publish() = %v", err)
	}
	h.waitUntil(t, "the group consumed the redelivery", func() bool { return h.consumedEverything(t, publishedInTotal) })

	if n := h.count(t, "dmpf_example_reservations"); n != 1 {
		t.Fatalf("reservations = %d after the redelivery, want 1 (DuplicateIgnored)", n)
	}
	if n := h.count(t, "dmpf_inbox"); n != 1 {
		t.Fatalf("inbox rows = %d after the redelivery, want 1", n)
	}
	if n := h.count(t, "dmpf_quarantine"); n != 0 {
		t.Fatalf("quarantine rows = %d after the redelivery, want 0 — an identical payload is a duplicate, not a collision", n)
	}

	var snapshot struct {
		Items int
	}
	if err := h.pool.QueryRow(context.Background(), "SELECT snapshot FROM dmpf_example_reservations WHERE order_id = $1", e2eOrder).Scan(&snapshot); err != nil {
		t.Fatalf("reservation snapshot: %v", err)
	}
	if snapshot.Items != e2eItems {
		t.Fatalf("reservation items = %d, want %d", snapshot.Items, e2eItems)
	}

	t.Log("4b. the consumer's own fact leaves too: ReservationConfirmed drained to its channel, nothing failed")
	h.waitUntil(t, "the ReservationConfirmed row is published", func() bool {
		row, ok := h.outboxRow(t, reservationConfirmedType)
		return ok && row.Status == "published"
	})
	if n := h.countWhere(t, "dmpf_outbox", "status = 'failed'"); n != 0 {
		t.Fatalf("outbox rows failed = %d, want 0: every destination the use cases author has a channel", n)
	}

	t.Log("5. read: the order is placed, outside any transaction")
	status2, body := h.call(t, http.MethodGet, server.URL+"/orders/"+e2eOrder, "")
	if status2 != http.StatusOK {
		t.Fatalf("GET order: status = %d, body %s", status2, body)
	}
	var order struct {
		Status string `json:"status"`
		Items  []any  `json:"items"`
	}
	if err := json.Unmarshal(body, &order); err != nil {
		t.Fatalf("GET order body %s: %v", body, err)
	}
	if order.Status != "placed" || len(order.Items) != 1 {
		t.Fatalf("order = %+v, want placed with 1 line", order)
	}
}
