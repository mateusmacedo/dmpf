//go:build integration

package appkit

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	dmpfapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-app"
	reservationsconsumer "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-app/example/reservations"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb/pg"
)

const (
	// Wait is the INB-17 ceiling the harness declares for the inbox.
	Wait = 2 * time.Second
	// MaxAttempts is the GAR-08 limit the harness declares.
	MaxAttempts = 2
)

// Harness is KIT-05: the consumer adapter composed with its concrete
// realizations, fed raw bytes at the protocol edge, observed at the effect
// edge — the four tables.
type Harness struct {
	Consumer dmpfapp.Consumer
	Pool     *pgxpool.Pool
}

// NewReservations composes the reservations consumer of dmpf-app over the
// Postgres the suite runs against, with the clock and identifiers the test
// injects (KIT-07).
func NewReservations(t testing.TB, clock dmpfports.Clock, ids dmpfports.IDGenerator) Harness {
	t.Helper()
	pool := pg.OpenPool(t)
	return Harness{
		Consumer: reservationsconsumer.NewConsumer(pool, clock, ids, Wait, MaxAttempts),
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
		reservationsconsumer.ConsumerName, a.messageID).Scan(&a.InboxAtAck)
}

func (a *Ack) Release(context.Context) error {
	a.Releases++
	return nil
}

// Deliver hands the raw bytes of one delivery to the adapter and returns what
// it did, with the acknowledger that saw the gesture. messageID only tells the
// acknowledger which inbox row to watch.
func (h Harness) Deliver(ctx context.Context, messageID string, raw []byte, attempt int) (dmpfapp.Outcome, *Ack, error) {
	ack := &Ack{pool: h.Pool, messageID: messageID}
	outcome, err := h.Consumer.Consume(ctx, dmpfapp.Delivery{Raw: raw, Attempt: attempt}, ack)
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
		reservationsconsumer.ConsumerName, messageID).Scan(&status, &lastError)
	if err != nil {
		t.Fatalf("appkit.InboxRow %s: %v", messageID, err)
	}
	return status, lastError
}

// RawOrderPlaced is the CloudEvent of an OrderPlaced as a producer publishes
// it: the bytes the harness delivers at the protocol edge.
func RawOrderPlaced(t testing.TB, messageID, orderID string, items int32) []byte {
	t.Helper()
	payload, typeURL, err := envelope.Pack(&eventv1.OrderPlaced{OrderId: orderID, ItemCount: items})
	if err != nil {
		t.Fatalf("appkit.RawOrderPlaced: Pack: %v", err)
	}
	ce, err := envelope.Encode(envelope.Envelope{
		ID:              messageID,
		Source:          "urn:lidercap:orders",
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
