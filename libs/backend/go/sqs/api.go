// comment-discipline-ok-file: arquivo de contrato interno; o godoc cita a regra de FND-06 (TRP-07) que o símbolo realiza, dentro do limite de 3 linhas.

package sqs

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// sqsAPI is the slice of *sqs.Client this provider uses, declared here so a
// test substitutes it.
type sqsAPI interface {
	SendMessage(ctx context.Context, in *sqs.SendMessageInput, opts ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, opts ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, in *sqs.ChangeMessageVisibilityInput, opts ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, opts ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	GetQueueAttributes(ctx context.Context, in *sqs.GetQueueAttributesInput, opts ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
}

// snsAPI is the slice of *sns.Client this provider uses.
type snsAPI interface {
	Publish(ctx context.Context, in *sns.PublishInput, opts ...func(*sns.Options)) (*sns.PublishOutput, error)
	GetSubscriptionAttributes(ctx context.Context, in *sns.GetSubscriptionAttributesInput, opts ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error)
	GetTopicAttributes(ctx context.Context, in *sns.GetTopicAttributesInput, opts ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error)
	ListSubscriptionsByTopic(ctx context.Context, in *sns.ListSubscriptionsByTopicInput, opts ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error)
}

var (
	_ sqsAPI = (*sqs.Client)(nil)
	_ snsAPI = (*sns.Client)(nil)
)

// NewSQSClient is the real client over the resolved AWS configuration, pointed
// at the endpoint when one is configured (an emulator, a VPC endpoint). A
// plaintext endpoint is logged, so the opt-out never passes unseen.
func NewSQSClient(cfg Config) *sqs.Client {
	warnPlaintext(cfg)
	return sqs.NewFromConfig(cfg.AWS, func(o *sqs.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = &cfg.Endpoint
		}
	})
}

// NewSNSClient is the real SNS client, likewise.
func NewSNSClient(cfg Config) *sns.Client {
	warnPlaintext(cfg)
	return sns.NewFromConfig(cfg.AWS, func(o *sns.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = &cfg.Endpoint
		}
	})
}

func warnPlaintext(cfg Config) {
	if cfg.Endpoint != "" && !strings.HasPrefix(cfg.Endpoint, "https://") {
		cfg.logger().Warn("sqs: endpoint without TLS by explicit development-only opt-out")
	}
}
