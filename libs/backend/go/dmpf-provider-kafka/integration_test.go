//go:build integration

package dmpfkafka_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/retry"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfkafka "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/channel"
)

const (
	messages = 100
	keys     = 3
)

// message is one published envelope: key k, sequence n within the key, and a
// payload whose hash the consumer recomputes.
func message(t *testing.T, key string, n int) ([]byte, string) {
	t.Helper()
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, uint64(n))
	env := envelope.Envelope{
		ID: fmt.Sprintf("evt-%s-%d", key, n), Source: "urn:dmpf:sales", SpecVersion: envelope.SpecVersion,
		Type: "sales.order.placed.v1", Subject: "order/" + key, Time: timestamppb.New(time.Now()),
		DataSchema: "type.googleapis.com/sales.order.v1.OrderPlaced", DataContentType: envelope.ContentType,
		CorrelationID: "corr", CausationID: "evt-0", PartitionKey: key,
		TraceParent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", Payload: payload,
	}
	raw, err := envelope.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return raw, payloadhash.Sum(payload)
}

// recordingSink acks everything, keeps the sequence seen per key and the
// hash of each payload, and can stop the run after a number of acks.
type recordingSink struct {
	mu       sync.Mutex
	perKey   map[string][]int
	hashes   map[string]string
	acks     int
	stopAt   int
	stop     context.CancelFunc
	failNext bool
}

func (s *recordingSink) Handle(ctx context.Context, raw []byte, _ int, ack dmpfports.Acknowledger) error {
	env, err := envelope.Unmarshal(raw)
	if err != nil {
		return err
	}
	n := int(binary.BigEndian.Uint64(env.Payload))
	s.mu.Lock()
	s.perKey[env.PartitionKey] = append(s.perKey[env.PartitionKey], n)
	s.hashes[env.ID] = payloadhash.Sum(env.Payload)
	s.acks++
	stopNow := s.stopAt > 0 && s.acks == s.stopAt
	s.mu.Unlock()
	if err := ack.Ack(ctx); err != nil {
		return err
	}
	if stopNow && s.stop != nil {
		// The crash: the run is cancelled right after this ack, before the
		// worker gets to commit anything further.
		s.stop()
	}
	return nil
}

func (s *recordingSink) total() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.acks
}

func integrationConfig(seeds []string, ch channel.Channel) dmpfkafka.Config {
	cfg := publishConfig()
	cfg.Brokers = seeds
	cfg.Clock = clock.System()
	cfg.Catalog = channel.Catalog{ch.Name: ch}
	return cfg
}

func integrationChannel(suffix string) channel.Channel {
	ch := ordersChannel()
	ch.Address = "dmpf-it-orders-" + suffix
	ch.Containment = "dmpf-it-orders-" + suffix + "-dlq"
	ch.Group = "dmpf-it-billing-" + suffix
	return ch
}

func runConsumer(t *testing.T, cfg dmpfkafka.Config, ch channel.Channel, sink *recordingSink) (context.CancelFunc, <-chan error) {
	t.Helper()
	c := &dmpfkafka.Consumer{
		Config: cfg, Channel: ch, Sink: sink,
		Backoff:            retry.Backoff{Base: 10 * time.Millisecond, Factor: 2, Cap: time.Second},
		ProcessingDeadline: 5 * time.Second, RebalanceTimeout: 30 * time.Second, QueuePerPartition: 32,
	}
	ctx, cancel := context.WithCancel(context.Background())
	sink.stop = cancel
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	return cancel, done
}

func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestIntegrationOrderPerKeyAndHashSurviveTheHop(t *testing.T) {
	seeds := brokers(t)
	ch := integrationChannel(uniqueSuffix())
	createTopics(t, seeds, int32(ch.Partitions), ch.Address, ch.Containment)
	cfg := integrationConfig(seeds, ch)

	pub, err := dmpfkafka.NewPublisher(cfg, nil)
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	defer pub.Close()

	published := map[string]string{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for n := range messages {
		key := fmt.Sprintf("k%d", n%keys)
		raw, hash := message(t, key, n/keys)
		published[fmt.Sprintf("evt-%s-%d", key, n/keys)] = hash
		if err := pub.Publish(ctx, ch.Name, raw); err != nil {
			t.Fatalf("Publish #%d: %v", n, err)
		}
	}

	sink := &recordingSink{perKey: map[string][]int{}, hashes: map[string]string{}}
	stop, done := runConsumer(t, cfg, ch, sink)
	waitFor(t, "all messages consumed", 60*time.Second, func() bool { return sink.total() >= messages })
	stop()
	<-done

	for key, seq := range sink.perKey {
		for i := range seq {
			if seq[i] != i {
				t.Fatalf("key %s consumed out of order: %v (KFK-06)", key, seq)
			}
		}
	}
	for id, hash := range published {
		if sink.hashes[id] != hash {
			t.Fatalf("payload hash of %s differs after the hop (ENV-18)", id)
		}
	}
}

func TestIntegrationAnInterruptedConsumerResumesWithoutLoss(t *testing.T) {
	seeds := brokers(t)
	ch := integrationChannel(uniqueSuffix())
	createTopics(t, seeds, int32(ch.Partitions), ch.Address, ch.Containment)
	cfg := integrationConfig(seeds, ch)

	pub, err := dmpfkafka.NewPublisher(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for n := range messages {
		raw, _ := message(t, fmt.Sprintf("k%d", n%keys), n/keys)
		if err := pub.Publish(ctx, ch.Name, raw); err != nil {
			t.Fatal(err)
		}
	}

	first := &recordingSink{perKey: map[string][]int{}, hashes: map[string]string{}, stopAt: 50}
	_, done := runConsumer(t, cfg, ch, first)
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("the first consumer did not stop after 50 acks")
	}

	second := &recordingSink{perKey: map[string][]int{}, hashes: map[string]string{}}
	stop, done := runConsumer(t, cfg, ch, second)
	seen := func() int {
		return len(first.hashes) + len(second.hashes) - overlap(first.hashes, second.hashes)
	}
	waitFor(t, "the second consumer to cover the rest", 60*time.Second, func() bool {
		first.mu.Lock()
		second.mu.Lock()
		defer first.mu.Unlock()
		defer second.mu.Unlock()
		return seen() >= messages
	})
	stop()
	<-done

	duplicates := overlap(first.hashes, second.hashes)
	// What the first consumer had acknowledged but not yet committed when it
	// stopped is redelivered: never more than one in-flight batch per partition.
	if duplicates > 32*keys {
		t.Fatalf("%d duplicates, want at most the in-flight batch", duplicates)
	}
	if seen() != messages {
		t.Fatalf("distinct messages seen = %d, want %d: a message was lost", seen(), messages)
	}
}

func overlap(a, b map[string]string) int {
	n := 0
	for id := range a {
		if _, dup := b[id]; dup {
			n++
		}
	}
	return n
}
