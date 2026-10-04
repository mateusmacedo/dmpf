// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (SQS-01, SQS-03b, SQS-05, SQS-06, SQS-12, TRP-18) ou FND-08 (RES-22) que o símbolo realiza, dentro do limite de 3 linhas.

package sqs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/compose"
)

// Limits of the transport the publisher checks before any send.
const (
	// MaxMessageBytes is the SQS and SNS limit on the final, encoded body (SQS-12).
	MaxMessageBytes = 256 * 1024
	// MaxAttributes is what raw SNS → SQS delivery carries without dropping (SQS-03b).
	MaxAttributes = 10
)

// AttrPublishedAt is the operational message attribute with the instant of
// publication (TRP-18); it never enters the envelope.
const AttrPublishedAt = "dmpf-published-at"

// message is what a channel's publisher sends: the body, the FIFO derivations
// when the channel is FIFO, and the operational attributes.
type message struct {
	body       string
	groupID    string
	dedupID    string
	attributes map[string]string
}

// prepare encodes the envelope once (SQS-01), derives group and dedup for a
// FIFO channel (SQS-05, SQS-06), and refuses a body over the path's smallest
// limit (SQS-12) or too many attributes (SQS-03b) before anything is sent.
func prepare(ch channel.Channel, raw []byte, now time.Time, extra map[string]string) (message, error) {
	env, err := envelope.Unmarshal(raw)
	if err != nil {
		return message{}, fmt.Errorf("%w: %s", ErrInvalidEnvelope, ch.Name)
	}
	body := EncodeBody(raw)
	if len(body) > MaxMessageBytes {
		return message{}, fmt.Errorf("%w: %s: %d bytes encoded", ErrMessageTooLarge, ch.Name, len(body))
	}
	attributes := map[string]string{AttrPublishedAt: now.UTC().Format(time.RFC3339Nano)}
	for k, v := range extra {
		attributes[k] = v
	}
	if len(attributes) > MaxAttributes {
		return message{}, fmt.Errorf("%w: %s: %d attributes", ErrTooManyAttributes, ch.Name, len(attributes))
	}
	if size := len(body) + attributesSize(attributes); size > MaxMessageBytes {
		return message{}, fmt.Errorf("%w: %s: %d bytes with attributes", ErrMessageTooLarge, ch.Name, size)
	}
	m := message{body: body, attributes: attributes}
	if IsFIFO(ch) {
		m.groupID = GroupID(env.PartitionKey)
		m.dedupID = DedupID(env.Source, env.ID, payloadhash.Sum(env.Payload))
	}
	return m, nil
}

// attributesSize is what SQS adds to the body when it applies the size limit:
// every attribute name and value (SQS-12).
func attributesSize(attributes map[string]string) int {
	size := 0
	for k, v := range attributes {
		size += len(k) + len(v)
	}
	return size
}

func sqsAttributes(attributes map[string]string) map[string]sqstypes.MessageAttributeValue {
	out := make(map[string]sqstypes.MessageAttributeValue, len(attributes))
	for k, v := range attributes {
		out[k] = sqstypes.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(v)}
	}
	return out
}

// Publisher realizes the relay's Publisher over SQS: one queue per catalogued
// channel, the envelope Base64-encoded once in the body, group and dedup
// derived for FIFO, and the send through the composition of RES-22.
type Publisher struct {
	cfg          Config
	api          sqsAPI
	call         resilience.Call
	ops          map[string]resilience.Operation
	destinations map[string][]attribute.KeyValue
}

// NewPublisher builds the publisher over an SQS client; NewSQSClient gives the
// real one.
func NewPublisher(cfg Config, api sqsAPI) (*Publisher, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if api == nil {
		return nil, fmt.Errorf("%w: sqs client", ErrIncompleteConfig)
	}
	call, err := compose.Build(composition(cfg))
	if err != nil {
		return nil, err
	}
	return &Publisher{
		cfg: cfg, api: api, call: call,
		ops:          operations(cfg, "publish", channel.SQS),
		destinations: destinations(cfg, semconv.MessagingSystemAWSSQS, channel.SQS),
	}, nil
}

// Publish routes the message by logical destination: the catalogue gives the
// queue URL, the envelope gives group and dedup, the body is the envelope
// encoded once. A channel bound to SNS → SQS is not this publisher's.
func (p *Publisher) Publish(ctx context.Context, destination string, raw []byte) error {
	ch, err := p.cfg.Channel(destination)
	if err != nil {
		return err
	}
	if ch.Transport != channel.SQS {
		return fmt.Errorf("%w: %s is %s; use the SNS publisher", ErrNotSQSChannel, ch.Name, ch.Transport)
	}
	m, err := prepare(ch, raw, p.cfg.Clock.Now(), nil)
	if err != nil {
		return err
	}
	in := &sqs.SendMessageInput{
		QueueUrl:          aws.String(ch.Address),
		MessageBody:       aws.String(m.body),
		MessageAttributes: sqsAttributes(m.attributes),
	}
	if m.groupID != "" {
		in.MessageGroupId, in.MessageDeduplicationId = aws.String(m.groupID), aws.String(m.dedupID)
	}
	return p.call(ctx, p.ops[ch.Name], func(ctx context.Context) error {
		annotate(ctx, p.destinations[ch.Name])
		_, err := p.api.SendMessage(ctx, in)
		return err
	})
}

func composition(cfg Config) compose.Config {
	return compose.Config{
		Sheet:          cfg.Sheet,
		Clock:          cfg.Clock,
		Tracer:         cfg.Tracer,
		Instruments:    cfg.Instruments,
		LoggerProvider: cfg.LoggerProvider,
		Rand:           cfg.Rand,
		Category:       categoryOf,
		Classifier:     Classifier,
	}
}

// operations is one resilience.Operation per catalogued channel of the given
// transports, built once: the publish path then allocates nothing for it.
func operations(cfg Config, prefix string, transports ...channel.Transport) map[string]resilience.Operation {
	cc := composition(cfg)
	ops := make(map[string]resilience.Operation, len(cfg.Catalog))
	for _, ch := range cfg.Catalog {
		for _, t := range transports {
			if ch.Transport == t {
				ops[ch.Name] = compose.Operation(cc, prefix+" "+ch.Name, true)
			}
		}
	}
	return ops
}

func destinations(cfg Config, system attribute.KeyValue, transport channel.Transport) map[string][]attribute.KeyValue {
	name := queueName
	if transport == channel.SNSSQS {
		name = topicName
	}
	out := make(map[string][]attribute.KeyValue, len(cfg.Catalog))
	for _, ch := range cfg.Catalog {
		if ch.Transport == transport {
			out[ch.Name] = destination(system, name(ch.Address))
		}
	}
	return out
}

func destination(system attribute.KeyValue, name string) []attribute.KeyValue {
	return []attribute.KeyValue{system, semconv.MessagingDestinationName(name)}
}

func topicName(topicARN string) string {
	return topicARN[strings.LastIndexAny(topicARN, ":/")+1:]
}

// The relay's send already carries its own messaging.* (app/relay/send.go);
// only the resilience span opened outside the relay takes these (RF-B7).
func annotate(ctx context.Context, attributes []attribute.KeyValue) {
	if _, owned := tracing.OwnsSpan(ctx); owned {
		return
	}
	trace.SpanFromContext(ctx).SetAttributes(attributes...)
}
