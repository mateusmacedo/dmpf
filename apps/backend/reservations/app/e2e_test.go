//go:build integration

package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	e2eOccurred = ports.Instant(1_757_000_000_000_000_000)
	e2eWait     = 2 * time.Second

	// e2eTimeout is the consumer's own time policy, which CTX-28 makes
	// mandatory: the adapter mounts a deadline per attempt from it.
	e2eTimeout  = 10 * time.Second
	e2eAttempts = 2
)

var errBoom = errors.New("boom")

type e2eClock struct{}

func (e2eClock) Now() ports.Instant { return e2eOccurred }

type sequenceIDs struct {
	mu   sync.Mutex
	next int
}

func (s *sequenceIDs) NewMessageID() ports.MessageID {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	return ports.MessageID(fmt.Sprintf("m-%06d", s.next))
}

// recordingAck reads the inbox at the moment of the Ack: a row already visible
// to another connection is the proof that the commit came first (INB-08).
type recordingAck struct {
	pool          *pgxpool.Pool
	acks          int
	releases      int
	inboxAtAck    int64
	messageIDSeen ports.MessageID
}

func (a *recordingAck) Ack(ctx context.Context) error {
	a.acks++
	return a.pool.QueryRow(ctx, "SELECT count(*) FROM inbox WHERE message_id = $1", string(a.messageIDSeen)).Scan(&a.inboxAtAck)
}

func (a *recordingAck) Release(context.Context) error {
	a.releases++
	return nil
}

type tableCounts struct{ inbox, reservations, outbox, quarantine int64 }

func counts(t *testing.T, pool *pgxpool.Pool) tableCounts {
	t.Helper()
	var c tableCounts
	const stmt = `SELECT
		(SELECT count(*) FROM inbox),
		(SELECT count(*) FROM reservations),
		(SELECT count(*) FROM outbox),
		(SELECT count(*) FROM quarantine)`
	if err := pool.QueryRow(context.Background(), stmt).Scan(&c.inbox, &c.reservations, &c.outbox, &c.quarantine); err != nil {
		t.Fatalf("counts: %v", err)
	}
	return c
}

func inboxRow(t *testing.T, pool *pgxpool.Pool, messageID string) (status string, lastError *string) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		"SELECT status, last_error FROM inbox WHERE consumer_name = $1 AND message_id = $2",
		app.ConsumerName, messageID).Scan(&status, &lastError)
	if err != nil {
		t.Fatalf("inbox row %s: %v", messageID, err)
	}
	return status, lastError
}

func quarantinedEnvelope(t *testing.T, pool *pgxpool.Pool, messageID string) ([]byte, string) {
	t.Helper()
	var raw []byte
	var reason string
	err := pool.QueryRow(context.Background(),
		"SELECT envelope, reason FROM quarantine WHERE consumer_name = $1 AND message_id = $2",
		app.ConsumerName, messageID).Scan(&raw, &reason)
	if err != nil {
		t.Fatalf("quarantine row %s: %v", messageID, err)
	}
	return raw, reason
}

func consume(t *testing.T, pool *pgxpool.Pool, consumer kernelapp.Consumer, messageID string, raw []byte, attempt int) (kernelapp.Outcome, error, *recordingAck) {
	t.Helper()
	ack := &recordingAck{pool: pool, messageIDSeen: ports.MessageID(messageID)}
	outcome, err := consumer.Consume(context.Background(), kernelapp.Delivery{Raw: raw, Attempt: attempt}, ack)
	return outcome, err, ack
}

// failingReservations wraps the real repository and fails Save from inside the
// same pgx.Tx, so the rollback observed is Postgres's own, not a fake's.
type failingReservations struct {
	ports.Repository[domain.OrderID, domain.Snapshot]
	err error
}

func (f failingReservations) Save(context.Context, domain.OrderID, domain.Snapshot, ports.Version) error {
	return f.err
}

type failingOutbox struct{ err error }

func (f failingOutbox) Enqueue(context.Context, ports.OutboxEntry) error { return f.err }

func consumerWith(pool *pgxpool.Pool, decorate func(application.Resources) application.Resources) kernelapp.Consumer {
	bind := func(tx *postgres.Tx) application.Resources {
		return decorate(app.Bind(e2eWait)(tx))
	}
	service := application.Service{
		UoW:       postgres.NewUnitOfWork(pool, bind),
		Clock:     e2eClock{},
		IDs:       &sequenceIDs{},
		Authorize: usecase.AllowAll[application.Operation](),
		Consumer:  app.ConsumerName,
	}
	return kernelapp.Consumer{
		Name:        app.ConsumerName,
		MaxAttempts: e2eAttempts,
		Handle:      app.Handler(service),
		Containment: postgres.NewQuarantine(pool),
		Clock:       e2eClock{},
		Timeout:     e2eTimeout,
		Boundary:    e2eBoundary,
		Locale:      "en",
	}
}

var e2eBoundary = kernelapp.Boundary{Transport: kernelapp.TransportDevelopmentOnly, Sources: []string{"urn:dmpf:orders"}}

func TestFirstReceptionAppliesConfirmsAndDerivesTheOutbox(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := app.NewConsumer(pool, e2eClock{}, &sequenceIDs{}, e2eWait, e2eTimeout, e2eAttempts, e2eBoundary)

	outcome, err, ack := consume(t, pool, consumer, "evt-1", appkit.RawOrderPlaced(t, "evt-1", "o-1", 2), 1)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if outcome.Disposition != usecase.R1D1 || outcome.Contained {
		t.Fatalf("outcome = %+v", outcome)
	}
	if got := counts(t, pool); got != (tableCounts{inbox: 1, reservations: 1, outbox: 1}) {
		t.Fatalf("counts = %+v, want one row in inbox, reservations and outbox (INB-07)", got)
	}
	if status, lastError := inboxRow(t, pool, "evt-1"); status != "processed" || lastError != nil {
		t.Fatalf("inbox = (%s, %v), want (processed, nil)", status, lastError)
	}
	var messageType string
	if err := pool.QueryRow(context.Background(), "SELECT message_type FROM outbox").Scan(&messageType); err != nil {
		t.Fatalf("outbox: %v", err)
	}
	if messageType != "com.company.reservations.reservation-confirmed.v1" {
		t.Fatalf("derived outbox message_type = %s", messageType)
	}
	if ack.acks != 1 || ack.releases != 0 || ack.inboxAtAck != 1 {
		t.Fatalf("ack=%d release=%d inboxAtAck=%d: the ack must follow the commit (INB-08)", ack.acks, ack.releases, ack.inboxAtAck)
	}
}

func TestBusinessRejectionCommitsRejectedAndConfirms(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := app.NewConsumer(pool, e2eClock{}, &sequenceIDs{}, e2eWait, e2eTimeout, e2eAttempts, e2eBoundary)

	outcome, err, ack := consume(t, pool, consumer, "evt-2", appkit.RawOrderPlaced(t, "evt-2", "o-2", 0), 1)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if outcome.Disposition != usecase.R1D2 || outcome.Contained {
		t.Fatalf("outcome = %+v", outcome)
	}
	if got := counts(t, pool); got != (tableCounts{inbox: 1}) {
		t.Fatalf("counts = %+v, want only the inbox row", got)
	}
	status, lastError := inboxRow(t, pool, "evt-2")
	if status != "rejected" || lastError == nil || *lastError != string(domain.CodeReservationNothingToReserve) {
		t.Fatalf("inbox = (%s, %v), want (rejected, %s)", status, lastError, domain.CodeReservationNothingToReserve)
	}
	if ack.acks != 1 || ack.inboxAtAck != 1 {
		t.Fatalf("ack=%d inboxAtAck=%d", ack.acks, ack.inboxAtAck)
	}
}

func TestRedeliveriesShortCircuit(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := app.NewConsumer(pool, e2eClock{}, &sequenceIDs{}, e2eWait, e2eTimeout, e2eAttempts, e2eBoundary)
	applied := appkit.RawOrderPlaced(t, "evt-1", "o-1", 2)
	rejected := appkit.RawOrderPlaced(t, "evt-2", "o-2", 0)
	for _, raw := range [][]byte{applied, rejected} {
		if _, err, _ := consume(t, pool, consumer, "seed", raw, 1); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	before := counts(t, pool)

	t.Run("R2 reapplies nothing", func(t *testing.T) {
		outcome, err, ack := consume(t, pool, consumer, "evt-1", applied, 2)
		if err != nil || outcome.Disposition != usecase.R2 || outcome.Contained {
			t.Fatalf("outcome = %+v, err = %v", outcome, err)
		}
		if ack.acks != 1 || counts(t, pool) != before {
			t.Fatalf("ack=%d counts=%+v, want ack and no new row", ack.acks, counts(t, pool))
		}
	})

	t.Run("R3 re-emits no rejection event", func(t *testing.T) {
		outcome, err, ack := consume(t, pool, consumer, "evt-2", rejected, 2)
		if err != nil || outcome.Disposition != usecase.R3 || outcome.Contained {
			t.Fatalf("outcome = %+v, err = %v", outcome, err)
		}
		if ack.acks != 1 || counts(t, pool) != before {
			t.Fatalf("ack=%d counts=%+v, want ack and no new outbox row (INB-12)", ack.acks, counts(t, pool))
		}
	})

	t.Run("R4 contains the collision byte for byte", func(t *testing.T) {
		collision := appkit.RawOrderPlaced(t, "evt-1", "o-1", 3)
		outcome, err, ack := consume(t, pool, consumer, "evt-1", collision, 1)
		if err != nil || outcome.Disposition != usecase.R4 || !outcome.Contained || outcome.Reason != ports.ReasonCollision {
			t.Fatalf("outcome = %+v, err = %v", outcome, err)
		}
		if ack.acks != 1 {
			t.Fatalf("ack=%d, want the message taken out of the flow", ack.acks)
		}
		raw, reason := quarantinedEnvelope(t, pool, "evt-1")
		if reason != string(ports.ReasonCollision) || !bytes.Equal(raw, collision) {
			t.Fatalf("quarantine = (%s, %d bytes), want the published bytes (GAR-07)", reason, len(raw))
		}
		after := counts(t, pool)
		if after.inbox != before.inbox || after.reservations != before.reservations || after.outbox != before.outbox {
			t.Fatalf("R4 wrote business state: %+v vs %+v", after, before)
		}
	})
}

func TestTransientFailureRollsBackAndReleases(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := consumerWith(pool, func(r application.Resources) application.Resources {
		r.Reservations = failingReservations{r.Reservations, usecase.NewFailure(usecase.TransientDependency, true, errBoom)}
		return r
	})

	outcome, err, ack := consume(t, pool, consumer, "evt-3", appkit.RawOrderPlaced(t, "evt-3", "o-3", 1), 1)
	if !errors.Is(err, errBoom) || outcome.Disposition != usecase.R1D3 || outcome.Contained {
		t.Fatalf("outcome = %+v, err = %v", outcome, err)
	}
	if got := counts(t, pool); got != (tableCounts{}) {
		t.Fatalf("counts = %+v, want nothing written under R1×D3", got)
	}
	if ack.releases != 1 || ack.acks != 0 {
		t.Fatalf("release=%d ack=%d", ack.releases, ack.acks)
	}
}

func TestExhaustedAttemptsContainThePoisonMessage(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := consumerWith(pool, func(r application.Resources) application.Resources {
		r.Reservations = failingReservations{r.Reservations, usecase.NewFailure(usecase.TransientDependency, true, errBoom)}
		return r
	})
	poison := appkit.RawOrderPlaced(t, "evt-4", "o-4", 1)

	outcome, err, ack := consume(t, pool, consumer, "evt-4", poison, e2eAttempts)
	if !errors.Is(err, errBoom) || !outcome.Contained || outcome.Reason != ports.ReasonAttemptsExhausted {
		t.Fatalf("outcome = %+v, err = %v, want attempts-exhausted (GAR-08)", outcome, err)
	}
	if ack.acks != 1 || ack.releases != 0 {
		t.Fatalf("ack=%d release=%d, want the poison message out of the flow", ack.acks, ack.releases)
	}
	if raw, _ := quarantinedEnvelope(t, pool, "evt-4"); !bytes.Equal(raw, poison) {
		t.Fatal("quarantine must keep the published bytes (GAR-07)")
	}

	// The next message of the same partition is not blocked by the poison one.
	healthy := app.NewConsumer(pool, e2eClock{}, &sequenceIDs{}, e2eWait, e2eTimeout, e2eAttempts, e2eBoundary)
	outcome, err, _ = consume(t, pool, healthy, "evt-5", appkit.RawOrderPlaced(t, "evt-5", "o-4", 1), 1)
	if err != nil || outcome.Disposition != usecase.R1D1 {
		t.Fatalf("the partition stayed blocked: outcome = %+v, err = %v", outcome, err)
	}
}

func TestTerminalFailureIsContained(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := consumerWith(pool, func(r application.Resources) application.Resources {
		r.Reservations = failingReservations{r.Reservations, usecase.NewFailure(usecase.Forbidden, false, errBoom)}
		return r
	})

	outcome, err, ack := consume(t, pool, consumer, "evt-6", appkit.RawOrderPlaced(t, "evt-6", "o-6", 1), 1)
	if !errors.Is(err, errBoom) || outcome.Disposition != usecase.R1D4 || outcome.Reason != ports.ReasonTerminalFailure {
		t.Fatalf("outcome = %+v, err = %v", outcome, err)
	}
	if got := counts(t, pool); got != (tableCounts{quarantine: 1}) {
		t.Fatalf("counts = %+v, want only the quarantine row", got)
	}
	if ack.acks != 1 {
		t.Fatalf("ack=%d", ack.acks)
	}
}

func TestOutboxFailureRollsBackDeduplicationAndState(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := consumerWith(pool, func(r application.Resources) application.Resources {
		r.Outbox = failingOutbox{errBoom}
		return r
	})

	_, err, _ := consume(t, pool, consumer, "evt-7", appkit.RawOrderPlaced(t, "evt-7", "o-7", 1), 1)
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if got := counts(t, pool); got.inbox != 0 || got.reservations != 0 || got.outbox != 0 {
		t.Fatalf("counts = %+v: deduplication, state and outbox must commit together or not at all (INB-07)", got)
	}
}

func TestInvalidEnvelopeIsContainedWithoutTouchingTheInbox(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := app.NewConsumer(pool, e2eClock{}, &sequenceIDs{}, e2eWait, e2eTimeout, e2eAttempts, e2eBoundary)
	garbage := []byte("definitely not a cloudevent")

	outcome, err, ack := consume(t, pool, consumer, "", garbage, 1)
	if err != nil || outcome.Classified || !outcome.Contained || outcome.Reason != ports.ReasonInvalidEnvelope {
		t.Fatalf("outcome = %+v, err = %v", outcome, err)
	}
	if got := counts(t, pool); got != (tableCounts{quarantine: 1}) {
		t.Fatalf("counts = %+v, want only the quarantine row (INB-10)", got)
	}
	if ack.acks != 1 {
		t.Fatalf("ack=%d", ack.acks)
	}
}

func TestPayloadOfAnotherContractIsTerminal(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := app.NewConsumer(pool, e2eClock{}, &sequenceIDs{}, e2eWait, e2eTimeout, e2eAttempts, e2eBoundary)
	payload, typeURL, err := envelope.Pack(&eventv1.ItemAdded{OrderId: "o-8", Sku: "sku", Quantity: 1})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	ce, err := envelope.Encode(envelope.Envelope{
		ID: "evt-8", Source: "urn:dmpf:orders", SpecVersion: envelope.SpecVersion,
		Type: "com.company.orders.item-added.v1", Subject: "order/o-8",
		Time:       timestamppb.New(time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)),
		DataSchema: typeURL, DataContentType: envelope.ContentType,
		CorrelationID: "corr-8", CausationID: "evt-8", PartitionKey: "o-8",
		TraceParent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	raw, err := proto.Marshal(ce)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	outcome, err, _ := consume(t, pool, consumer, "evt-8", raw, 1)
	if !errors.Is(err, envelope.ErrSchemaMismatch) || outcome.Disposition != usecase.R1D4 || outcome.Reason != ports.ReasonTerminalFailure {
		t.Fatalf("outcome = %+v, err = %v", outcome, err)
	}
	if got := counts(t, pool); got != (tableCounts{quarantine: 1}) {
		t.Fatalf("counts = %+v", got)
	}
}

func TestRedeliveryWithANewMessageIDDoesNotDuplicateTheEffect(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := app.NewConsumer(pool, e2eClock{}, &sequenceIDs{}, e2eWait, e2eTimeout, e2eAttempts, e2eBoundary)
	if _, err, _ := consume(t, pool, consumer, "evt-1", appkit.RawOrderPlaced(t, "evt-1", "o-1", 2), 1); err != nil {
		t.Fatalf("first: %v", err)
	}

	// V32: the inbox sees a new identity and classifies R1; only the natural
	// key of the effect (the order) keeps the reservation from doubling (GAR-04, GAR-10).
	outcome, err, ack := consume(t, pool, consumer, "evt-9", appkit.RawOrderPlaced(t, "evt-9", "o-1", 2), 1)
	if err != nil || outcome.Disposition != usecase.R1D2 {
		t.Fatalf("outcome = %+v, err = %v, want R1×D2 from the already-reserved rule", outcome, err)
	}
	if got := counts(t, pool); got != (tableCounts{inbox: 2, reservations: 1, outbox: 1}) {
		t.Fatalf("counts = %+v, want two inbox rows and still one reservation and one event", got)
	}
	if status, lastError := inboxRow(t, pool, "evt-9"); status != "rejected" || lastError == nil || *lastError != string(domain.CodeReservationAlreadyReserved) {
		t.Fatalf("inbox evt-9 = (%s, %v)", status, lastError)
	}
	if ack.acks != 1 {
		t.Fatalf("ack=%d", ack.acks)
	}
}

func TestSignalsExposeTheConsumerSide(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := app.NewConsumer(pool, e2eClock{}, &sequenceIDs{}, e2eWait, e2eTimeout, e2eAttempts, e2eBoundary)
	if _, err, _ := consume(t, pool, consumer, "evt-1", appkit.RawOrderPlaced(t, "evt-1", "o-1", 2), 1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err, _ := consume(t, pool, consumer, "evt-1", appkit.RawOrderPlaced(t, "evt-1", "o-1", 5), 1); err != nil {
		t.Fatalf("collision: %v", err)
	}
	if _, err, _ := consume(t, pool, consumer, "", []byte("garbage"), 1); err != nil {
		t.Fatalf("invalid: %v", err)
	}

	signals, err := postgres.InboxSignals(context.Background(), pool, app.ConsumerName)
	if err != nil {
		t.Fatalf("InboxSignals: %v", err)
	}
	if signals.QuarantineDepth != 2 || signals.Collisions != 1 || signals.InvalidEnvelopes != 1 {
		t.Fatalf("signals = %+v (GAR-12)", signals)
	}
}

// CTX-27 end to end: an OrderPlaced that is intact and well-formed, but from a
// producer the boundary does not admit, writes nothing but its quarantine row.
func TestAnOrderPlacedFromOutsideTheBoundaryWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := app.NewConsumer(pool, e2eClock{}, &sequenceIDs{}, e2eWait, e2eTimeout, e2eAttempts, e2eBoundary)
	payload, typeURL, err := envelope.Pack(&eventv1.OrderPlaced{OrderId: "o-9", ItemCount: 1})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	tenant := "acme"
	ce, err := envelope.Encode(envelope.Envelope{
		ID: "evt-9", Source: "urn:dmpf:intruder", SpecVersion: envelope.SpecVersion,
		Type: "com.company.orders.order-placed.v1", Subject: "order/o-9",
		Time:       timestamppb.New(time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)),
		DataSchema: typeURL, DataContentType: envelope.ContentType,
		CorrelationID: "corr-9", CausationID: "evt-9", PartitionKey: "o-9",
		TraceParent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		TenantID:    &tenant,
		Payload:     payload,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	raw, err := proto.Marshal(ce)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	outcome, err, _ := consume(t, pool, consumer, "evt-9", raw, 1)
	if err != nil || !outcome.Contained || outcome.Reason != ports.ReasonUntrustedBoundary || outcome.Classified {
		t.Fatalf("outcome = %+v, err = %v; want contained as untrusted-boundary before any context", outcome, err)
	}
	if got := counts(t, pool); got != (tableCounts{quarantine: 1}) {
		t.Fatalf("counts = %+v, want only the quarantine row", got)
	}
}

// CTX-26 over the seven dispositions: a platform chain carries no tenant, and a
// reservation is tenant data, so step 1 denies before any unit of work (IDN-07,
// IDN-08) instead of widening the query. With no predicate that makes it
// retryable the refusal is terminal: contained once, nothing written.
func TestAPlatformChainOrderPlacedIsRefusedTerminallyWithoutWriting(t *testing.T) {
	pool := appkit.OpenPool(t)
	consumer := app.NewConsumer(pool, e2eClock{}, &sequenceIDs{}, e2eWait, e2eTimeout, e2eAttempts, e2eBoundary)

	outcome, err, ack := consume(t, pool, consumer, "evt-10", appkit.RawOrderPlacedWithoutTenant(t, "evt-10", "o-10", 1), 1)
	if !errors.Is(err, ports.ErrDenied) || outcome.Disposition != usecase.R1D4 || outcome.Reason != ports.ReasonTerminalFailure {
		t.Fatalf("outcome = %+v, err = %v; want R1xD4 over the step 1 denial", outcome, err)
	}
	if got := counts(t, pool); got != (tableCounts{quarantine: 1}) {
		t.Fatalf("counts = %+v, want only the quarantine row: no inbox, no reservation, no outbox", got)
	}
	if ack.acks != 1 || ack.releases != 0 {
		t.Fatalf("ack=%d release=%d, want it taken out of the flow on the first attempt", ack.acks, ack.releases)
	}
}
