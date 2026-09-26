//go:build integration

package sqs_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
)

// endpoint reads the harness variable and skips outside the CI: without an
// emulator there is nothing to prove, and the CI sets it (ci.yml, floci).
func endpoint(t *testing.T) string {
	t.Helper()
	value := os.Getenv("SQS_ENDPOINT")
	if value == "" {
		t.Skip("SQS_ENDPOINT is not set: the integration tests need an SQS/SNS emulator")
	}
	return value
}

func awsConfig(t *testing.T) aws.Config {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		t.Fatalf("LoadDefaultConfig: %v", err)
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return cfg
}

func integrationConfig(t *testing.T) provider.Config {
	t.Helper()
	cfg := validConfig()
	cfg.AWS = awsConfig(t)
	cfg.Endpoint = endpoint(t)
	cfg.InsecureForDevelopmentOnly = !strings.HasPrefix(cfg.Endpoint, "https://")
	cfg.Clock = clock.System()
	cfg.Rand = nil
	return cfg
}

func uniqueSuffix() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

// createQueue creates a queue, with a redrive to dlq when given, and deletes
// it when the test ends. FIFO queues are named by the .fifo suffix.
func createQueue(t *testing.T, api *sqs.Client, name string, dlqARN string, maxReceive int) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	attrs := map[string]string{}
	if len(name) > 5 && name[len(name)-5:] == ".fifo" {
		attrs["FifoQueue"] = "true"
	}
	if dlqARN != "" {
		attrs["RedrivePolicy"] = fmt.Sprintf(`{"deadLetterTargetArn":%q,"maxReceiveCount":"%d"}`, dlqARN, maxReceive)
	}
	out, err := api.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(name), Attributes: attrs})
	if err != nil {
		t.Fatalf("CreateQueue(%s): %v", name, err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = api.DeleteQueue(ctx, &sqs.DeleteQueueInput{QueueUrl: out.QueueUrl})
	})
	return *out.QueueUrl
}

func queueARN(t *testing.T, api *sqs.Client, url string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := api.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{QueueUrl: aws.String(url), AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn}})
	if err != nil {
		t.Fatalf("GetQueueAttributes: %v", err)
	}
	return out.Attributes["QueueArn"]
}

func messagesInQueue(t *testing.T, api *sqs.Client, url string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := api.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{QueueUrl: aws.String(url), AttributeNames: []sqstypes.QueueAttributeName{
		sqstypes.QueueAttributeNameApproximateNumberOfMessages, sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
	}})
	if err != nil {
		t.Fatalf("GetQueueAttributes: %v", err)
	}
	var n int
	fmt.Sscan(out.Attributes["ApproximateNumberOfMessages"], &n)
	var invisible int
	fmt.Sscan(out.Attributes["ApproximateNumberOfMessagesNotVisible"], &invisible)
	return n + invisible
}

func createTopic(t *testing.T, api *sns.Client, name string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := api.CreateTopic(ctx, &sns.CreateTopicInput{Name: aws.String(name)})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = api.DeleteTopic(ctx, &sns.DeleteTopicInput{TopicArn: out.TopicArn})
	})
	return *out.TopicArn
}

func subscribe(t *testing.T, api *sns.Client, topicARN, queueARN string, raw bool) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := api.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: aws.String(topicARN), Protocol: aws.String("sqs"), Endpoint: aws.String(queueARN),
		Attributes:            map[string]string{"RawMessageDelivery": fmt.Sprint(raw)},
		ReturnSubscriptionArn: true,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	return *out.SubscriptionArn
}

func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
