package dmpfsqs

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func NewAcknowledger(api sqsAPI, queueURL, receipt string, backoff time.Duration, stop func()) *acknowledger {
	return newAcknowledger(api, queueURL, receipt, backoff, stop)
}

type Acknowledger = acknowledger

func VisibilitySeconds(d time.Duration) int32 { return visibilitySeconds(d) }

type SQSAPI = sqsAPI

type SNSAPI = snsAPI

// Call is one recorded API call: the operation and its input.
type Call struct {
	Op    string
	Input any
}

// FakeSQS records every call and answers ReceiveMessage from a script.
type FakeSQS struct {
	mu       sync.Mutex
	calls    []Call
	receives chan *sqs.ReceiveMessageOutput
	Attrs    map[string]string

	SendErr    error
	DeleteErr  error
	ChangeErr  error
	ReceiveErr error

	// ChangeGate, when set, blocks every ChangeMessageVisibility until closed:
	// it lets a test hold a heartbeat tick in flight.
	ChangeGate chan struct{}
}

var _ sqsAPI = (*FakeSQS)(nil)

func NewFakeSQS() *FakeSQS {
	return &FakeSQS{receives: make(chan *sqs.ReceiveMessageOutput, 64), Attrs: map[string]string{}}
}

func (f *FakeSQS) record(op string, in any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, Call{Op: op, Input: in})
}

// Calls is a copy of the recorded calls, optionally filtered by operation.
func (f *FakeSQS) Calls(op string) []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Call, 0, len(f.calls))
	for _, c := range f.calls {
		if op == "" || c.Op == op {
			out = append(out, c)
		}
	}
	return out
}

// Deliver queues one ReceiveMessage answer.
func (f *FakeSQS) Deliver(out *sqs.ReceiveMessageOutput) { f.receives <- out }

func (f *FakeSQS) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.record("SendMessage", in)
	if f.SendErr != nil {
		return nil, f.SendErr
	}
	id := "m-" + time.Now().Format("150405.000000000")
	return &sqs.SendMessageOutput{MessageId: &id}, nil
}

func (f *FakeSQS) ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	f.record("ReceiveMessage", in)
	if f.ReceiveErr != nil {
		return nil, f.ReceiveErr
	}
	select {
	case out := <-f.receives:
		return out, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(20 * time.Millisecond):
		return &sqs.ReceiveMessageOutput{}, nil
	}
}

func (f *FakeSQS) ChangeMessageVisibility(ctx context.Context, in *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
	f.record("ChangeMessageVisibility", in)
	if f.ChangeGate != nil {
		select {
		case <-f.ChangeGate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.ChangeErr != nil {
		return nil, f.ChangeErr
	}
	return &sqs.ChangeMessageVisibilityOutput{}, nil
}

func (f *FakeSQS) DeleteMessage(_ context.Context, in *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.record("DeleteMessage", in)
	if f.DeleteErr != nil {
		return nil, f.DeleteErr
	}
	return &sqs.DeleteMessageOutput{}, nil
}

func (f *FakeSQS) GetQueueAttributes(_ context.Context, in *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	f.record("GetQueueAttributes", in)
	return &sqs.GetQueueAttributesOutput{Attributes: f.Attrs}, nil
}

// FakeSNS records publishes and answers attribute lookups from maps; ByTopic
// lists the subscriptions the broker would report for a topic.
type FakeSNS struct {
	mu            sync.Mutex
	calls         []Call
	Subscriptions map[string]map[string]string
	Topics        map[string]map[string]string
	ByTopic       map[string][]snstypes.Subscription
	PublishErr    error
}

var _ snsAPI = (*FakeSNS)(nil)

func NewFakeSNS() *FakeSNS {
	return &FakeSNS{Subscriptions: map[string]map[string]string{}, Topics: map[string]map[string]string{}, ByTopic: map[string][]snstypes.Subscription{}}
}

func (f *FakeSNS) Calls(op string) []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Call, 0, len(f.calls))
	for _, c := range f.calls {
		if op == "" || c.Op == op {
			out = append(out, c)
		}
	}
	return out
}

func (f *FakeSNS) Publish(_ context.Context, in *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	f.mu.Lock()
	f.calls = append(f.calls, Call{Op: "Publish", Input: in})
	f.mu.Unlock()
	if f.PublishErr != nil {
		return nil, f.PublishErr
	}
	id := "n-1"
	return &sns.PublishOutput{MessageId: &id}, nil
}

func (f *FakeSNS) GetSubscriptionAttributes(_ context.Context, in *sns.GetSubscriptionAttributesInput, _ ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error) {
	f.mu.Lock()
	f.calls = append(f.calls, Call{Op: "GetSubscriptionAttributes", Input: in})
	f.mu.Unlock()
	return &sns.GetSubscriptionAttributesOutput{Attributes: f.Subscriptions[*in.SubscriptionArn]}, nil
}

func (f *FakeSNS) GetTopicAttributes(_ context.Context, in *sns.GetTopicAttributesInput, _ ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	f.mu.Lock()
	f.calls = append(f.calls, Call{Op: "GetTopicAttributes", Input: in})
	f.mu.Unlock()
	return &sns.GetTopicAttributesOutput{Attributes: f.Topics[*in.TopicArn]}, nil
}

func (f *FakeSNS) ListSubscriptionsByTopic(_ context.Context, in *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	f.mu.Lock()
	f.calls = append(f.calls, Call{Op: "ListSubscriptionsByTopic", Input: in})
	f.mu.Unlock()
	return &sns.ListSubscriptionsByTopicOutput{Subscriptions: f.ByTopic[*in.TopicArn]}, nil
}

// Subscribe registers a subscription on the fake, with its attributes.
func (f *FakeSNS) Subscribe(topicARN, subscriptionARN, protocol string, attrs map[string]string) {
	f.ByTopic[topicARN] = append(f.ByTopic[topicARN], snstypes.Subscription{
		SubscriptionArn: &subscriptionARN, TopicArn: &topicARN, Protocol: &protocol,
	})
	f.Subscriptions[subscriptionARN] = attrs
}
