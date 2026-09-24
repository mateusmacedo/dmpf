//go:build integration

package appkit

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app"
)

const (
	// Wait is the INB-17 ceiling the harness declares for the inbox.
	Wait = 2 * time.Second
	// MaxAttempts is the GAR-08 limit the harness declares.
	MaxAttempts = 2
	// Timeout is the consumer's own time policy, which CTX-28 makes mandatory.
	Timeout = 10 * time.Second
)

// Harness is KIT-05: the consumer adapter composed with its concrete
// realizations, fed raw bytes at the protocol edge, observed at the effect
// edge — the four tables.
type Harness struct {
	Consumer kernel.Consumer
	Pool     *pgxpool.Pool
}

// NewReservations composes the reservations consumer of app over the
// Postgres the suite runs against, with the clock and identifiers the test
// injects (KIT-07).
func NewReservations(t testing.TB, clock ports.Clock, ids ports.IDGenerator) Harness {
	t.Helper()
	pool := pg.OpenPool(t)
	return Harness{
		Consumer: app.NewConsumer(pool, clock, ids, Wait, Timeout, MaxAttempts, Boundary),
		Pool:     pool,
	}
}

// Ack records the broker gesture the adapter applied and reads the inbox at
// the moment of the ack: a row already visible to another connection is the
// proof that the commit came first (INB-08).
type Ack struct {
	pool      *pgxpool.Pool
	messageID string

	Acks       int
	Releases   int
	InboxAtAck int64
}

func (a *Ack) Ack(ctx context.Context) error {
	a.Acks++
	return a.pool.QueryRow(ctx,
		"SELECT count(*) FROM dmpf_inbox WHERE consumer_name = $1 AND message_id = $2",
		app.ConsumerName, a.messageID).Scan(&a.InboxAtAck)
}

func (a *Ack) Release(context.Context) error {
	a.Releases++
	return nil
}

// Deliver hands the raw bytes of one delivery to the adapter and returns what
// it did, with the acknowledger that saw the gesture. messageID only tells the
// acknowledger which inbox row to watch.
func (h Harness) Deliver(ctx context.Context, messageID string, raw []byte, attempt int) (kernel.Outcome, *Ack, error) {
	ack := &Ack{pool: h.Pool, messageID: messageID}
	outcome, err := h.Consumer.Consume(ctx, kernel.Delivery{Raw: raw, Attempt: attempt}, ack)
	return outcome, ack, err
}

// Effects is the effect edge: how many rows each table holds.
type Effects struct {
	Inbox, Reservations, Outbox, Quarantine int64
}

// queryTimeout bounds each read of the effect edge; a Postgres that stops
// answering fails the test by name instead of letting it hang.
const queryTimeout = 10 * time.Second

func (h Harness) Effects(t testing.TB) Effects {
	t.Helper()
	var e Effects
	const stmt = `SELECT
		(SELECT count(*) FROM dmpf_inbox),
		(SELECT count(*) FROM dmpf_example_reservations),
		(SELECT count(*) FROM dmpf_outbox),
		(SELECT count(*) FROM dmpf_quarantine)`
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	if err := h.Pool.QueryRow(ctx, stmt).Scan(&e.Inbox, &e.Reservations, &e.Outbox, &e.Quarantine); err != nil {
		t.Fatalf("appkit.Effects: %v", err)
	}
	return e
}

// InboxRow reads the committed status and last error of one message of the
// reservations consumer.
func (h Harness) InboxRow(t testing.TB, messageID string) (status string, lastError *string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	err := h.Pool.QueryRow(ctx,
		"SELECT status, last_error FROM dmpf_inbox WHERE consumer_name = $1 AND message_id = $2",
		app.ConsumerName, messageID).Scan(&status, &lastError)
	if err != nil {
		t.Fatalf("appkit.InboxRow %s: %v", messageID, err)
	}
	return status, lastError
}

// Tenant is the scope the producer resolved, carried on every event this
// harness publishes. A consumer that writes a scoped aggregate needs it: the
// relay puts it on the envelope (CTX-13), and without it persistence refuses.
const Tenant = "acme"

// RawOrderPlaced is the CloudEvent of an OrderPlaced as a producer publishes
// it: the bytes the harness delivers at the protocol edge.
func RawOrderPlaced(t testing.TB, messageID, orderID string, items int32) []byte {
	t.Helper()
	return rawOrderPlaced(t, messageID, orderID, items, ptr(Tenant))
}

// RawOrderPlacedWithoutTenant is the platform chain of CTX-26: an event whose
// producer resolved no tenant. It is not malformed — it is the shape ENV-12
// gives absence — so what refuses it is persistence, not the envelope.
func RawOrderPlacedWithoutTenant(t testing.TB, messageID, orderID string, items int32) []byte {
	t.Helper()
	return rawOrderPlaced(t, messageID, orderID, items, nil)
}

func ptr[T any](v T) *T { return &v }

// OrdersSource is the producer the harness publishes as, and Boundary the
// consumer's trust in it, so a harness message crosses the boundary like a
// production one does (CTX-27).
const OrdersSource = "urn:dmpf:orders"

var Boundary = kernel.Boundary{Transport: kernel.TransportDevelopmentOnly, Sources: []string{OrdersSource}}

func rawOrderPlaced(t testing.TB, messageID, orderID string, items int32, tenant *string) []byte {
	t.Helper()
	payload, typeURL, err := envelope.Pack(&eventv1.OrderPlaced{OrderId: orderID, ItemCount: items})
	if err != nil {
		t.Fatalf("appkit.RawOrderPlaced: Pack: %v", err)
	}
	ce, err := envelope.Encode(envelope.Envelope{
		ID:              messageID,
		Source:          OrdersSource,
		SpecVersion:     envelope.SpecVersion,
		Type:            "com.company.orders.order-placed.v1",
		Subject:         "order/" + orderID,
		Time:            timestamppb.New(time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)),
		DataSchema:      typeURL,
		DataContentType: envelope.ContentType,
		CorrelationID:   "corr-" + messageID,
		CausationID:     messageID,
		PartitionKey:    orderID,
		TraceParent:     "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		TenantID:        tenant,
		Payload:         payload,
	})
	if err != nil {
		t.Fatalf("appkit.RawOrderPlaced: Encode: %v", err)
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(ce)
	if err != nil {
		t.Fatalf("appkit.RawOrderPlaced: Marshal: %v", err)
	}
	return raw
}
