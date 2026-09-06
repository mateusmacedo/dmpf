// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (SQS-11, TRP-30) ou FND-04 (GAR-07) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfsqs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/attempt"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/compose"
)

// Operational attributes the dead letter carries beside the envelope, so a
// replay knows why, from whom and when the message left the flow (GAR-07).
const (
	AttrReason      = "dmpf-reason"
	AttrConsumer    = "dmpf-consumer"
	AttrError       = "dmpf-error"
	AttrMessageID   = "dmpf-message-id"
	AttrContainedAt = "dmpf-contained-at"
	AttrAttempt     = attempt.Header
)

// AttributeValueLimit bounds the two attribute values that come from outside
// the provider — the message id of the received envelope and the adapter's
// error — so a hostile or broken value cannot make the quarantine fail.
const AttributeValueLimit = 1024

// DLQ realizes dmpfports.Containment by explicit publication on the channel's
// containment queue (SQS-11, D4/R4 and invalid envelope): the envelope goes
// encoded once and intact, and the caller deletes only after it returns (TRP-30).
type DLQ struct {
	cfg  Config
	ch   channel.Channel
	api  sqsAPI
	call resilience.Call
	op   resilience.Operation
}

var _ dmpfports.Containment = (*DLQ)(nil)

// NewDLQ builds the containment for one channel over an SQS client.
func NewDLQ(cfg Config, api sqsAPI, ch channel.Channel) (*DLQ, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if api == nil {
		return nil, fmt.Errorf("%w: sqs client", ErrIncompleteConfig)
	}
	if err := ch.Validate(); err != nil {
		return nil, err
	}
	if ch.Transport != channel.SQS && ch.Transport != channel.SNSSQS {
		return nil, fmt.Errorf("%w: %s", ErrNotSQSChannel, ch.Name)
	}
	cc := composition(cfg, "quarantine")
	call, err := compose.Build(cc)
	if err != nil {
		return nil, err
	}
	return &DLQ{cfg: cfg, ch: ch, api: api, call: call, op: compose.Operation(cc, "quarantine "+ch.Name, true)}, nil
}

// Quarantine sends the contained envelope to the containment queue with the
// attempt from the context (TRP-52); a FIFO containment derives group and dedup
// from the envelope when it decodes, else a group of its own, deduplicated by bytes.
func (d *DLQ) Quarantine(ctx context.Context, c dmpfports.Contained) error {
	if c.Consumer == "" || c.Reason == "" || len(c.Envelope) == 0 {
		return ErrInvalidContainment
	}
	body := EncodeBody(c.Envelope)
	attributes := map[string]string{
		AttrReason:      string(c.Reason),
		AttrConsumer:    c.Consumer,
		AttrMessageID:   bounded(string(c.MessageID)),
		AttrContainedAt: time.Unix(0, int64(c.At)).UTC().Format(time.RFC3339Nano),
	}
	if c.Error != "" {
		attributes[AttrError] = bounded(c.Error)
	}
	if n, ok := attempt.FromContext(ctx); ok {
		attributes[AttrAttempt] = attempt.Encode(n)
	}
	if size := len(body) + attributesSize(attributes); size > MaxMessageBytes {
		return fmt.Errorf("%w: containment of %s: %d bytes with attributes", ErrMessageTooLarge, d.ch.Name, size)
	}
	in := &sqs.SendMessageInput{
		QueueUrl:          aws.String(d.ch.Containment),
		MessageBody:       aws.String(body),
		MessageAttributes: sqsAttributes(attributes),
	}
	if IsFIFO(channel.Channel{Address: d.ch.Containment}) {
		group, dedup := "invalid-envelope", digest(string(c.Consumer), body)
		if env, err := envelope.Unmarshal(c.Envelope); err == nil {
			group = GroupID(env.PartitionKey)
			dedup = DedupID(env.Source, env.ID, payloadhash.Sum(env.Payload))
		}
		in.MessageGroupId, in.MessageDeduplicationId = aws.String(group), aws.String(dedup)
	}
	return d.call(ctx, d.op, func(ctx context.Context) error {
		_, err := d.api.SendMessage(ctx, in)
		return err
	})
}

// bounded cuts a value at AttributeValueLimit bytes, on a rune boundary.
func bounded(s string) string {
	if len(s) <= AttributeValueLimit {
		return s
	}
	return strings.ToValidUTF8(s[:AttributeValueLimit], "")
}
