//go:build integration

package dmpfsqs_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/retry"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfsqs "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-sqs"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

// countingSink applies one gesture to every message and counts deliveries.
type countingSink struct {
	gesture    func(ctx context.Context, ack dmpfports.Acknowledger) error
	deliveries atomic.Int32
	attempts   chan int
}

func (s *countingSink) Handle(ctx context.Context, _ []byte, attempt int, ack dmpfports.Acknowledger) error {
	s.deliveries.Add(1)
	select {
	case s.attempts <- attempt:
	default:
	}
	return s.gesture(ctx, ack)
}

func integrationConsumer(cfg dmpfsqs.Config, ch channel.Channel, queueURL string, sink dmpfsqs.Sink) *dmpfsqs.Consumer {
	return &dmpfsqs.Consumer{
		Config: cfg, Channel: ch, QueueURL: queueURL, Sink: sink,
		Backoff:            retry.Backoff{Base: time.Second, Factor: 1, Cap: time.Second},
		VisibilityBase:     10 * time.Second,
		HeartbeatEvery:     3 * time.Second,
		ProcessingDeadline: 30 * time.Second,
		Concurrency:        2,
		WaitTime:           time.Second,
	}
}

func TestIntegrationFIFORedrivesToTheDLQOnTheThirdDelivery(t *testing.T) {
	cfg := integrationConfig(t)
	api := dmpfsqs.NewSQSClient(cfg)
	suffix := uniqueSuffix()
	dlqURL := createQueue(t, api, "dmpf-it-dlq-"+suffix+".fifo", "", 0)
	queueURL := createQueue(t, api, "dmpf-it-"+suffix+".fifo", queueARN(t, api, dlqURL), 2)

	ch := fifoChannel()
	ch.Address, ch.Containment = queueURL, dlqURL
	cfg.Catalog = channel.Catalog{ch.Name: ch}

	pub, err := dmpfsqs.NewPublisher(cfg, api)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := validRaw(t, "k1")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := pub.Publish(ctx, ch.Name, raw); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// Every delivery is released with a short backoff: the queue redelivers,
	// counts, and the redrive policy moves the message on the third receipt.
	sink := &countingSink{attempts: make(chan int, 8), gesture: func(ctx context.Context, ack dmpfports.Acknowledger) error { return ack.Release(ctx) }}
	consumer := integrationConsumer(cfg, ch, queueURL, sink)
	runCtx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(runCtx, api) }()

	waitFor(t, "the message to reach the DLQ", 60*time.Second, func() bool { return messagesInQueue(t, api, dlqURL) >= 1 })
	stop()
	<-done

	if got := sink.deliveries.Load(); got != 2 {
		t.Fatalf("deliveries = %d, want 2 before the redrive (maxReceiveCount 2, SQS-11 line 1)", got)
	}
	first, second := <-sink.attempts, <-sink.attempts
	if first != 1 || second != 2 {
		t.Fatalf("attempts = %d, %d; want the receive count 1 then 2 (TRP-52)", first, second)
	}
	if messagesInQueue(t, api, queueURL) != 0 {
		t.Fatal("the source queue still holds the message after the redrive")
	}
}

func TestIntegrationAckDeletesAndEmptiesTheQueue(t *testing.T) {
	cfg := integrationConfig(t)
	api := dmpfsqs.NewSQSClient(cfg)
	suffix := uniqueSuffix()
	queueURL := createQueue(t, api, "dmpf-it-std-"+suffix, "", 0)
	dlqURL := createQueue(t, api, "dmpf-it-std-dlq-"+suffix, "", 0)

	ch := standardChannel()
	ch.Address, ch.Containment = queueURL, dlqURL
	cfg.Catalog = channel.Catalog{ch.Name: ch}

	pub, err := dmpfsqs.NewPublisher(cfg, api)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for range 5 {
		raw, _ := validRaw(t, "k1")
		if err := pub.Publish(ctx, ch.Name, raw); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "5 messages in the queue", 10*time.Second, func() bool { return messagesInQueue(t, api, queueURL) == 5 })

	received := make(chan []byte, 8)
	sink := &countingSink{attempts: make(chan int, 8), gesture: func(ctx context.Context, ack dmpfports.Acknowledger) error { return ack.Ack(ctx) }}
	wrapped := sinkFunc(func(ctx context.Context, raw []byte, attempt int, ack dmpfports.Acknowledger) error {
		received <- raw
		return sink.Handle(ctx, raw, attempt, ack)
	})
	consumer := integrationConsumer(cfg, ch, queueURL, wrapped)
	runCtx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- consumer.Run(runCtx, api) }()

	waitFor(t, "the queue to drain", 60*time.Second, func() bool { return sink.deliveries.Load() >= 5 && messagesInQueue(t, api, queueURL) == 0 })
	stop()
	<-done

	raw := <-received
	if _, err := envelope.Unmarshal(raw); err != nil {
		t.Fatalf("the consumer handed the sink something that is not the envelope: %v (SQS-01)", err)
	}
}

type sinkFunc func(ctx context.Context, raw []byte, attempt int, ack dmpfports.Acknowledger) error

func (f sinkFunc) Handle(ctx context.Context, raw []byte, attempt int, ack dmpfports.Acknowledger) error {
	return f(ctx, raw, attempt, ack)
}

func TestIntegrationSNSToSQSRawDeliveryIsRequiredAndPreservesTheEnvelope(t *testing.T) {
	cfg := integrationConfig(t)
	sqsAPI := dmpfsqs.NewSQSClient(cfg)
	snsAPI := dmpfsqs.NewSNSClient(cfg)
	suffix := uniqueSuffix()

	rawQueue := createQueue(t, sqsAPI, "dmpf-it-sns-raw-"+suffix, "", 0)
	wrappedQueue := createQueue(t, sqsAPI, "dmpf-it-sns-wrapped-"+suffix, "", 0)
	topicARN := createTopic(t, snsAPI, "dmpf-it-topic-"+suffix)
	subscribe(t, snsAPI, topicARN, queueARN(t, sqsAPI, rawQueue), true)
	wrappedSub := subscribe(t, snsAPI, topicARN, queueARN(t, sqsAPI, wrappedQueue), false)

	ch := snsChannel()
	ch.Address, ch.Containment = topicARN, rawQueue
	cfg.Catalog = channel.Catalog{ch.Name: ch}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// The hop without raw delivery is refused at construction: the publisher
	// lists the topic's subscriptions itself (SQS-02).
	if _, err := dmpfsqs.NewSNSPublisher(ctx, cfg, snsAPI); !errors.Is(err, dmpfsqs.ErrRawDeliveryRequired) {
		t.Fatalf("NewSNSPublisher(topic with a wrapped subscription) = %v, want ErrRawDeliveryRequired", err)
	}
	// The negative row is produced outside the provider: the wrapped queue
	// still receives what the SDK publishes on the topic, and only then the
	// wrapped subscription is removed so the provider accepts the topic.
	raw, env := validRaw(t, "k1")
	if _, err := snsAPI.Publish(ctx, &sns.PublishInput{TopicArn: aws.String(topicARN), Message: aws.String(dmpfsqs.EncodeBody(raw))}); err != nil {
		t.Fatalf("Publish (sdk): %v", err)
	}
	if _, err := snsAPI.Unsubscribe(ctx, &sns.UnsubscribeInput{SubscriptionArn: aws.String(wrappedSub)}); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	pub, err := dmpfsqs.NewSNSPublisher(ctx, cfg, snsAPI)
	if err != nil {
		t.Fatalf("NewSNSPublisher(raw only) = %v", err)
	}
	if err := pub.Publish(ctx, ch.Name, raw); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	receive := func(url string) string {
		var body string
		waitFor(t, "a message on "+url, 30*time.Second, func() bool {
			out, err := sqsAPI.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{QueueUrl: aws.String(url), MaxNumberOfMessages: 1, WaitTimeSeconds: 2})
			if err != nil || len(out.Messages) == 0 {
				return false
			}
			body = aws.ToString(out.Messages[0].Body)
			return true
		})
		return body
	}

	// Positive row: the raw subscription delivers the envelope's encoding.
	decoded, err := dmpfsqs.DecodeBody(receive(rawQueue))
	if err != nil {
		t.Fatalf("DecodeBody(raw hop) = %v", err)
	}
	got, err := envelope.Unmarshal(decoded)
	if err != nil || got.ID != env.ID || got.PartitionKey != env.PartitionKey {
		t.Fatalf("the raw hop did not preserve the envelope: %v", err)
	}

	// Negative row: the same publication reaches the wrapped subscription
	// inside the notification, which the decoder refuses (SQS-02, §5.2).
	if _, err := dmpfsqs.DecodeBody(receive(wrappedQueue)); !errors.Is(err, dmpfsqs.ErrSNSEnvelopeNotRaw) {
		t.Fatalf("DecodeBody(wrapped hop) = %v, want ErrSNSEnvelopeNotRaw", err)
	}
	var _ sqstypes.Message
	var _ *sns.Client = snsAPI
}
