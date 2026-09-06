// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (KFK-12, TRP-13, TRP-30) ou FND-04 (GAR-07) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfkafka

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/attempt"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/compose"
)

// Operational headers the dead-letter record carries beside the envelope, so a
// replay knows why, from whom and when the message left the flow (GAR-07).
const (
	HeaderReason      = "dmpf-reason"
	HeaderConsumer    = "dmpf-consumer"
	HeaderError       = "dmpf-error"
	HeaderMessageID   = "dmpf-message-id"
	HeaderContainedAt = "dmpf-contained-at"
	HeaderAttempt     = attempt.Header
)

// HeaderValueLimit bounds the two header values that come from outside the
// provider — the message id of the received envelope and the adapter's error —
// so a hostile or broken value cannot inflate the dead letter.
const HeaderValueLimit = 1024

// DLQ realizes dmpfports.Containment over the channel's dead-letter topic
// (KFK-12): the envelope goes byte for byte (TRP-13) with the diagnosis in
// headers, and the caller advances the offset only after it returns (TRP-30).
type DLQ struct {
	cfg    Config
	ch     channel.Channel
	client client
	call   resilience.Call
	op     resilience.Operation
}

var _ dmpfports.Containment = (*DLQ)(nil)

// NewDLQ opens a producer for the channel's containment topic.
func NewDLQ(cfg Config, ch channel.Channel) (*DLQ, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := ch.Validate(); err != nil {
		return nil, err
	}
	cl, err := newClient(cfg, kgo.RequiredAcks(kgo.AllISRAcks()))
	if err != nil {
		return nil, fmt.Errorf("dmpfkafka: dlq producer: %w", err)
	}
	d, err := newDLQ(cfg, ch, cl)
	if err != nil {
		cl.Close()
		return nil, err
	}
	return d, nil
}

func newDLQ(cfg Config, ch channel.Channel, cl client) (*DLQ, error) {
	if ch.Transport != channel.Kafka {
		return nil, fmt.Errorf("%w: %s", ErrNotKafkaChannel, ch.Name)
	}
	cc := composition(cfg, "quarantine")
	call, err := compose.Build(cc)
	if err != nil {
		return nil, err
	}
	return &DLQ{cfg: cfg, ch: ch, client: cl, call: call, op: compose.Operation(cc, "quarantine "+ch.Name, true)}, nil
}

// Quarantine publishes the contained envelope, intact, to the dead-letter
// topic with the attempt from the context (TRP-52); the key is the envelope's
// partition key when it still decodes, so one aggregate's dead letters stay together.
func (d *DLQ) Quarantine(ctx context.Context, c dmpfports.Contained) error {
	if c.Consumer == "" || c.Reason == "" || len(c.Envelope) == 0 {
		return ErrInvalidContainment
	}

	var key []byte
	if env, err := envelope.Unmarshal(c.Envelope); err == nil {
		key = []byte(env.PartitionKey)
	}
	headers := []kgo.RecordHeader{
		{Key: HeaderReason, Value: []byte(c.Reason)},
		{Key: HeaderConsumer, Value: []byte(c.Consumer)},
		{Key: HeaderMessageID, Value: []byte(bounded(string(c.MessageID)))},
		{Key: HeaderContainedAt, Value: []byte(time.Unix(0, int64(c.At)).UTC().Format(time.RFC3339Nano))},
	}
	if c.Error != "" {
		headers = append(headers, kgo.RecordHeader{Key: HeaderError, Value: []byte(bounded(c.Error))})
	}
	if n, ok := attempt.FromContext(ctx); ok {
		headers = append(headers, kgo.RecordHeader{Key: HeaderAttempt, Value: []byte(attempt.Encode(n))})
	}
	record := &kgo.Record{Topic: d.ch.Containment, Key: key, Value: c.Envelope, Headers: headers}

	return d.call(ctx, d.op, func(ctx context.Context) error {
		return d.client.ProduceSync(ctx, record).FirstErr()
	})
}

// bounded cuts a value at HeaderValueLimit bytes, on a rune boundary.
func bounded(s string) string {
	if len(s) <= HeaderValueLimit {
		return s
	}
	return strings.ToValidUTF8(s[:HeaderValueLimit], "")
}

// Close releases the producer.
func (d *DLQ) Close() { d.client.Close() }
