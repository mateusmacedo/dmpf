//go:build integration && distributed

package distkit

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-app"
	reservationsconsumer "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-app/example/reservations"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/reservations"
	obsclock "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfkafka "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-kafka"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/appkit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/ids"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

const (
	destination = "orders"
	at          = dmpfports.Instant(1_757_000_000_000_000_000)
)

// RunRole is the body a test package gives its TestDistkitRole: it reads the
// role before anything else and skips when there is none, so the function is
// inert in every run that is not a re-executed child.
func RunRole(t *testing.T) {
	t.Helper()
	role := Role(os.Getenv(EnvRole))
	if role == "" {
		t.Skip(EnvRole + " unset: this test only runs as a re-executed child of distkit.Harness")
	}
	cfg, ch := childConfig(t)
	plan := childPlan(t)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	switch role {
	case RoleProducer:
		produce(t, ctx, cfg, plan)
	case RoleConsumer:
		pool := openPool(t)
		consume(t, ctx, cfg, ch, adapterSink{consumer: reservationsconsumer.NewConsumer(pool, clock.New(at), &ids.Sequence{Prefix: "m-"}, appkit.Wait, appkit.MaxAttempts)})
	case RoleNaiveConsumer:
		consume(t, ctx, cfg, ch, naiveSink{pool: openPool(t)})
	default:
		t.Fatalf("distkit: unknown role %q", role)
	}
}

// openPool connects without resetting: the tables belong to the parent.
func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), tb.Env(t, "DMPF_PG_DSN"))
	if err != nil {
		t.Fatalf("distkit: pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// childConfig is the Kafka side of the child: the channel of KRN-10's own
// tests with the run's unique names, a development-only transport, and the
// real clock — the transport waits on timers a fake could not advance from
// another process.
func childConfig(t *testing.T) (dmpfkafka.Config, channel.Channel) {
	t.Helper()
	ch := channel.Channel{
		Name:          destination,
		Transport:     channel.Kafka,
		Address:       tb.Env(t, EnvTopic),
		EventType:     "com.company.orders.order-placed",
		ContractMajor: 1,
		ContractRef:   "company/orders/event/v1/order_placed.proto#OrderPlaced",
		Ordering:      channel.Ordering{Key: "partitionkey", Unit: channel.Partition},
		Redelivery: channel.KafkaWindow(channel.KafkaRetention{
			RetentionByTime: 7 * 24 * time.Hour, RetentionBySize: -1, CleanupPolicy: "delete", InitialOffset: "earliest",
		}),
		Containment: tb.Env(t, EnvDLQ),
		Retry:       channel.Retry{Strategy: channel.InlineWithLimit, MaxAttempts: appkit.MaxAttempts},
		Group:       tb.Env(t, EnvGroup),
		Partitions:  1,
		Partitioner: "default",
		KeyEncoding: "utf-8",
	}
	cfg := dmpfkafka.Config{
		Brokers:                    strings.Split(tb.Env(t, EnvBrokers), ","),
		InsecureForDevelopmentOnly: true,
		Catalog:                    channel.Catalog{ch.Name: ch},
		Sheet:                      resilience.Defaults("kafka"),
		Service:                    "dmpf-testkit",
		Clock:                      obsclock.System(),
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("distkit: config: %v", err)
	}
	return cfg, ch
}

// childPlan is the plan the parent serialized into the environment: the child
// never falls back to Default, so the two sides cannot disagree.
func childPlan(t *testing.T) Plan {
	t.Helper()
	var plan Plan
	if err := json.Unmarshal([]byte(tb.Env(t, EnvPlan)), &plan); err != nil {
		t.Fatalf("distkit: decode %s: %v", EnvPlan, err)
	}
	return plan
}

func produce(t *testing.T, ctx context.Context, cfg dmpfkafka.Config, plan Plan) {
	t.Helper()
	pub, err := dmpfkafka.NewPublisher(cfg, nil)
	if err != nil {
		t.Fatalf("distkit: NewPublisher: %v", err)
	}
	defer pub.Close()
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for _, id := range plan.Deliveries {
		if err := pub.Publish(ctx, destination, appkit.RawOrderPlaced(t, id, plan.Order, plan.Items)); err != nil {
			t.Fatalf("distkit: Publish %s: %v", id, err)
		}
	}
}

func consume(t *testing.T, ctx context.Context, cfg dmpfkafka.Config, ch channel.Channel, sink dmpfkafka.Sink) {
	t.Helper()
	c := &dmpfkafka.Consumer{
		Config: cfg, Channel: ch, Sink: sink,
		Backoff:            retry.Backoff{Base: 10 * time.Millisecond, Factor: 2, Cap: time.Second},
		ProcessingDeadline: 5 * time.Second, RebalanceTimeout: 30 * time.Second, QueuePerPartition: 32,
	}
	if err := c.Run(ctx); err != nil && ctx.Err() == nil {
		t.Fatalf("distkit: consumer: %v", err)
	}
}

// adapterSink is the bridge of FND-06 §11 between the transport and the
// consumer adapter. The adapter records its gesture on the acknowledger after
// its transaction (TRP-26), and that gesture — not the error — is what the
// worker applies; the error goes back whole so the worker can log the cause of
// an acknowledged failure or of a delivery left without a gesture.
type adapterSink struct{ consumer dmpfapp.Consumer }

func (s adapterSink) Handle(ctx context.Context, raw []byte, attempt int, ack dmpfports.Acknowledger) error {
	_, err := s.consumer.Consume(ctx, dmpfapp.Delivery{Raw: raw, Attempt: attempt}, ack)
	return err
}

// naiveSink is the negative vector of V32: it applies the effect on every
// delivery, with no inbox and no natural key — a redelivery doubles it.
type naiveSink struct{ pool *pgxpool.Pool }

const naiveUpsert = `
INSERT INTO dmpf_example_reservations (order_id, version, snapshot) VALUES ($1, 1, $2)
ON CONFLICT (order_id) DO UPDATE
   SET version  = dmpf_example_reservations.version + 1,
       snapshot = jsonb_set(dmpf_example_reservations.snapshot, '{Items}',
                            to_jsonb((dmpf_example_reservations.snapshot->>'Items')::int + $3))`

func (s naiveSink) Handle(ctx context.Context, raw []byte, _ int, ack dmpfports.Acknowledger) error {
	env, err := envelope.Unmarshal(raw)
	if err != nil {
		return err
	}
	var placed eventv1.OrderPlaced
	if err := envelope.Unpack(env, &placed); err != nil {
		return err
	}
	snapshot, err := json.Marshal(reservations.Snapshot{Order: reservations.OrderID(placed.GetOrderId()), Items: int(placed.GetItemCount()), Status: reservations.Confirmed})
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, naiveUpsert, placed.GetOrderId(), snapshot, placed.GetItemCount()); err != nil {
		return err
	}
	return ack.Ack(ctx)
}
