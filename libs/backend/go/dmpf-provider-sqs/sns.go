// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (SQS-02, SQS-03c, SQS-12, TRP-08) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfsqs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/channel"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/compose"
)

// SNSPublisher publishes an SNS → SQS channel: the same body, group and dedup
// as the queue publisher, on a topic whose SQS subscriptions were all found and
// verified to deliver raw (SQS-02) and, when FIFO, to carry no filter (SQS-03c).
type SNSPublisher struct {
	cfg  Config
	api  snsAPI
	call resilience.Call
	ops  map[string]resilience.Operation
}

// NewSNSPublisher verifies, at construction, every SQS subscription the broker
// itself reports for each SNS → SQS topic of the catalogue: at least one, all
// delivering raw (SQS-02), none with a filter policy on a FIFO topic (SQS-03c).
func NewSNSPublisher(ctx context.Context, cfg Config, api snsAPI) (*SNSPublisher, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if api == nil {
		return nil, fmt.Errorf("%w: sns client", ErrIncompleteConfig)
	}
	for _, ch := range cfg.Catalog {
		if ch.Transport != channel.SNSSQS {
			continue
		}
		if err := verifyTopic(ctx, api, ch); err != nil {
			return nil, err
		}
	}
	call, err := compose.Build(composition(cfg, "publish"))
	if err != nil {
		return nil, err
	}
	return &SNSPublisher{cfg: cfg, api: api, call: call, ops: operations(cfg, "publish", channel.SNSSQS)}, nil
}

// verifyTopic lists the topic's subscriptions and checks each SQS one.
func verifyTopic(ctx context.Context, api snsAPI, ch channel.Channel) error {
	fifo := IsFIFO(ch)
	var token *string
	sqsSubscriptions := 0
	for {
		page, err := api.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{TopicArn: aws.String(ch.Address), NextToken: token})
		if err != nil {
			return fmt.Errorf("dmpfsqs: %s: %w", ch.Name, err)
		}
		for _, sub := range page.Subscriptions {
			if aws.ToString(sub.Protocol) != "sqs" {
				continue
			}
			sqsSubscriptions++
			arn := aws.ToString(sub.SubscriptionArn)
			attrs, err := api.GetSubscriptionAttributes(ctx, &sns.GetSubscriptionAttributesInput{SubscriptionArn: aws.String(arn)})
			if err != nil {
				return fmt.Errorf("dmpfsqs: %s: %w", arn, err)
			}
			if attrs.Attributes["RawMessageDelivery"] != "true" {
				return fmt.Errorf("%w: %s: %s", ErrRawDeliveryRequired, ch.Name, arn)
			}
			if fifo && attrs.Attributes["FilterPolicy"] != "" {
				return fmt.Errorf("%w: %s: %s", ErrFilterOnFifoTopic, ch.Name, arn)
			}
		}
		if page.NextToken == nil || aws.ToString(page.NextToken) == "" {
			break
		}
		token = page.NextToken
	}
	if sqsSubscriptions == 0 {
		return fmt.Errorf("%w: %s: %s", ErrNoSubscriptions, ch.Name, ch.Address)
	}
	return nil
}

// Publish sends to the channel's topic. The size limit is the smallest of the
// path — topic and queue share the same 256 KiB here (SQS-12).
func (p *SNSPublisher) Publish(ctx context.Context, destination string, raw []byte) error {
	ch, err := p.cfg.Channel(destination)
	if err != nil {
		return err
	}
	if ch.Transport != channel.SNSSQS {
		return fmt.Errorf("%w: %s is %s; use the SQS publisher", ErrNotSQSChannel, ch.Name, ch.Transport)
	}
	m, err := prepare(ch, raw, p.cfg.Clock.Now(), nil)
	if err != nil {
		return err
	}
	attributes := make(map[string]snstypes.MessageAttributeValue, len(m.attributes))
	for k, v := range m.attributes {
		attributes[k] = snstypes.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(v)}
	}
	in := &sns.PublishInput{
		TopicArn:          aws.String(ch.Address),
		Message:           aws.String(m.body),
		MessageAttributes: attributes,
	}
	if m.groupID != "" {
		in.MessageGroupId, in.MessageDeduplicationId = aws.String(m.groupID), aws.String(m.dedupID)
	}
	return p.call(ctx, p.ops[ch.Name], func(ctx context.Context) error {
		_, err := p.api.Publish(ctx, in)
		return err
	})
}
