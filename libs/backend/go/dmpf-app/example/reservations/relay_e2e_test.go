//go:build integration

package reservationsconsumer_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-app"
	reservationsconsumer "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-app/example/reservations"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-app/relay"
	dmpfapplication "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
	orderspg "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres/example/orders"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/appkit"
)

const (
	relaySource      = "urn:lidercap:orders"
	relayMessageID   = "m-relay-0001"
	relayOrderID     = "o-relay-1"
	relayItems       = 4
	relayContextMeta = `{"correlationid":"corr-relay","causationid":"caus-relay","traceparent":"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"}`
)

// capturingPublisher keeps every delivery so the test can replay the bytes the
// relay actually put on the wire into the KRN-07 consumer.
type capturingPublisher struct {
	mu        sync.Mutex
	delivered [][]byte
	entered   chan struct{}
	once      sync.Once
}

func (p *capturingPublisher) Publish(_ context.Context, _ string, message []byte) error {
	p.mu.Lock()
	p.delivered = append(p.delivered, message)
	p.mu.Unlock()

	if p.entered != nil {
		p.once.Do(func() { close(p.entered) })
	}
	return nil
}

func (p *capturingPublisher) messages() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][]byte(nil), p.delivered...)
}

// crashingStore is the failure window of FND-04 §5.4 made deliberate: the
// message reaches the broker and the transition never lands. That is the
// at-least-once window, and this is what forces the republication of V32.
type crashingStore struct {
	relay.Store
	failMarkPublished bool
}

func (s *crashingStore) MarkPublished(ctx context.Context, id int64, claimID string) (int64, error) {
	if s.failMarkPublished {
		return 0, context.Canceled
	}
	return s.Store.MarkPublished(ctx, id, claimID)
}

// outboxOnly is the resource set this test's unit of work binds: the relay
// drains what the writer committed, and the writer here needs nothing else.
type outboxOnly struct{ Outbox dmpfports.Outbox }

func enqueueOrderPlaced(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	uow := dmpfpostgres.NewUnitOfWork(pool, func(tx *dmpfpostgres.Tx) outboxOnly {
		return outboxOnly{Outbox: tx.Outbox(orderspg.Mapper{})}
	})
	err := uow.Within(context.Background(), func(ctx context.Context, r outboxOnly) error {
		return r.Outbox.Enqueue(ctx, dmpfports.OutboxEntry{
			MessageID:        relayMessageID,
			OccurredAt:       e2eOccurred,
			Intent:           dmpfports.PublishIntent{Destination: "orders.integration", PartitionKey: relayOrderID},
			AggregateType:    "order",
			AggregateID:      relayOrderID,
			AggregateVersion: 1,
			Event:            orders.OrderPlaced{Order: relayOrderID, Items: relayItems},
		})
	})
	if err != nil {
		t.Fatalf("Enqueue() = %v, want nil", err)
	}

	// KRN-06 writes metadata as "{}" on purpose: the three context attributes
	// have no column of their own and producing them is KRN-09's. Until then a
	// record without them is not claimable, so the test supplies them.
	if _, err := pool.Exec(context.Background(),
		`UPDATE dmpf_outbox SET metadata = $2::jsonb WHERE message_id = $1`, relayMessageID, relayContextMeta); err != nil {
		t.Fatalf("inject metadata = %v, want nil", err)
	}
}

func relayConfig() relay.Config {
	return relay.Config{
		Source:         relaySource,
		Interval:       10 * time.Millisecond,
		BatchSize:      10,
		Lease:          150 * time.Millisecond,
		Concurrency:    2,
		MaxAttempts:    3,
		BackoffBase:    time.Millisecond,
		BackoffCeiling: 10 * time.Millisecond,
		ShutdownGrace:  2 * time.Second,
	}
}

func outboxStatus(t *testing.T, pool *pgxpool.Pool) (status string, attempts int) {
	t.Helper()

	err := pool.QueryRow(context.Background(),
		`SELECT status, attempt_count FROM dmpf_outbox WHERE message_id = $1`, relayMessageID).Scan(&status, &attempts)
	if err != nil {
		t.Fatalf("read outbox = %v, want nil", err)
	}
	return status, attempts
}

func TestTheRelayDrainsWhatTheWriterCommitted(t *testing.T) {
	pool := openPool(t)
	enqueueOrderPlaced(t, pool)

	publisher := &capturingPublisher{}
	drain, err := reservationsconsumer.NewRelay(pool, publisher, relayConfig())
	if err != nil {
		t.Fatalf("NewRelay() = %v, want nil", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if err := drain.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	messages := publisher.messages()
	if len(messages) != 1 {
		t.Fatalf("published %d messages, want 1", len(messages))
	}

	env, err := envelope.Unmarshal(messages[0])
	if err != nil {
		t.Fatalf("the published bytes are not a valid envelope: %v", err)
	}
	if env.ID != relayMessageID {
		t.Errorf("id = %q, want %q", env.ID, relayMessageID)
	}
	if env.Subject != relayOrderID {
		t.Errorf("subject = %q, want %q (ENV-14 maps it from aggregate_id)", env.Subject, relayOrderID)
	}
	if env.Source != relaySource {
		t.Errorf("source = %q, want %q", env.Source, relaySource)
	}

	if status, attempts := outboxStatus(t, pool); status != "published" || attempts != 1 {
		t.Fatalf("outbox row = (%s, %d attempts), want (published, 1)", status, attempts)
	}
}

// V32: a failure between publishing and marking republishes the message in the
// next cycle. The duplicate is expected under at-least-once — it is not a
// defect to be fixed here; it is the property the inbox of KRN-07 absorbs.
func TestAFailureBetweenPublishingAndMarkingRepublishesAndTheInboxAbsorbsIt(t *testing.T) {
	pool := openPool(t)
	enqueueOrderPlaced(t, pool)

	publisher := &capturingPublisher{entered: make(chan struct{})}
	config := relayConfig()

	crashing, err := reservationsconsumer.NewRelay(pool, publisher, config)
	if err != nil {
		t.Fatalf("NewRelay() = %v, want nil", err)
	}
	crashing.Store = &crashingStore{Store: crashing.Store, failMarkPublished: true}

	crashCtx, crashCancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer crashCancel()
	if err := crashing.Run(crashCtx); err != nil {
		t.Fatalf("first Run() = %v, want nil", err)
	}

	if len(publisher.messages()) == 0 {
		t.Fatal("the message never reached the broker in the first cycle")
	}
	if status, _ := outboxStatus(t, pool); status != "publishing" {
		t.Fatalf("outbox row = %s, want publishing: the transition must not have landed", status)
	}

	// The lease has to expire before the record is claimable again; that delay
	// is exactly what OBX-09 buys, and here it is 150ms.
	time.Sleep(config.Lease + 50*time.Millisecond)

	healthy, err := reservationsconsumer.NewRelay(pool, publisher, config)
	if err != nil {
		t.Fatalf("NewRelay() = %v, want nil", err)
	}
	retryCtx, retryCancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer retryCancel()
	if err := healthy.Run(retryCtx); err != nil {
		t.Fatalf("second Run() = %v, want nil", err)
	}

	messages := publisher.messages()
	if len(messages) < 2 {
		t.Fatalf("published %d times, want at least 2: the window must republish", len(messages))
	}

	first, err := envelope.Unmarshal(messages[0])
	if err != nil {
		t.Fatalf("first delivery = %v, want a valid envelope", err)
	}
	last, err := envelope.Unmarshal(messages[len(messages)-1])
	if err != nil {
		t.Fatalf("republished delivery = %v, want a valid envelope", err)
	}
	if first.ID != last.ID {
		t.Fatalf("republished under id %q, want the original %q", last.ID, first.ID)
	}

	if status, _ := outboxStatus(t, pool); status != "published" {
		t.Fatalf("outbox row = %s, want published after the retry", status)
	}

	// The consumer of KRN-07 receives both deliveries. The first does the work;
	// the second is absorbed by the inbox, which is what makes the duplicate
	// harmless rather than a second reservation.
	consumer := reservationsconsumer.NewConsumer(pool, fixedClock{}, &sequenceIDs{}, e2eWait, e2eAttempts)
	raw := appkit.RawOrderPlaced(t, relayMessageID, relayOrderID, relayItems)

	for delivery := 1; delivery <= 2; delivery++ {
		ack := &recordingAck{pool: pool, messageIDSeen: relayMessageID}
		outcome, err := consumer.Consume(context.Background(), dmpfapp.Delivery{Raw: raw, Attempt: delivery}, ack)
		if err != nil {
			t.Fatalf("delivery %d = %v, want nil", delivery, err)
		}
		if ack.acks != 1 {
			t.Fatalf("delivery %d acked %d times, want 1", delivery, ack.acks)
		}
		if delivery == 2 && outcome.Disposition != dmpfapplication.R2 {
			t.Fatalf("the republished delivery was classified %v, want R2 (already processed)", outcome.Disposition)
		}
	}

	if got := counts(t, pool); got.reservations != 1 {
		t.Fatalf("%d reservations after two deliveries of the same message, want 1", got.reservations)
	}
}
