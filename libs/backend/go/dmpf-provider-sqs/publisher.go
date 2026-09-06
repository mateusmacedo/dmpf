// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (SQS-01, SQS-03b, SQS-05, SQS-06, SQS-12, TRP-18) ou FND-08 (RES-22) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfsqs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/compose"
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
	cfg  Config
	api  sqsAPI
	call resilience.Call
	ops  map[string]resilience.Operation
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
	call, err := compose.Build(composition(cfg, "publish"))
	if err != nil {
		return nil, err
	}
	return &Publisher{cfg: cfg, api: api, call: call, ops: operations(cfg, "publish", channel.SQS)}, nil
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
		_, err := p.api.SendMessage(ctx, in)
		return err
	})
}

// composition is what dmpf-transport/compose needs from this provider, with the
// SDK error code as the failure category and the SDK classifier.
func composition(cfg Config, prefix string) compose.Config {
	return compose.Config{
		Sheet:       cfg.Sheet,
		Service:     cfg.Service,
		SpanPrefix:  "dmpf.sqs." + prefix + " ",
		Clock:       cfg.Clock,
		Tracer:      cfg.Tracer,
		Instruments: cfg.Instruments,
		Logger:      cfg.Logger,
		Rand:        cfg.Rand,
		Category:    categoryOf,
		Classifier:  Classifier,
	}
}

// operations is one resilience.Operation per catalogued channel of the given
// transports, built once: the publish path then allocates nothing for it.
func operations(cfg Config, prefix string, transports ...channel.Transport) map[string]resilience.Operation {
	cc := composition(cfg, prefix)
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
