// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (KFK-05, TRP-11, TRP-13, TRP-17, TRP-18) ou FND-08 (RES-22) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfkafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/channel"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/compose"
)

// HeaderPublishedAt is the operational header with the instant of production
// (TRP-18); it never enters the envelope.
const HeaderPublishedAt = "dmpf-published-at"

// ErrInvalidEnvelope is what Publish reports when it cannot read the partition
// key: what does not decode as the envelope of FND-05 is not published.
var ErrInvalidEnvelope = errors.New("dmpfkafka: message is not a valid envelope")

// Publisher realizes the relay's Publisher over Kafka: the record key is the
// envelope's partition key (KFK-05, TRP-11) and the value is the envelope byte
// for byte (TRP-13), produced through the composition of RES-22.
type Publisher struct {
	cfg      Config
	client   client
	call     resilience.Call
	observer Observer
	ops      map[string]resilience.Operation
}

// NewPublisher opens the producer — acks from all in-sync replicas, idempotent
// writes enabled by default — and composes its call. observer may be nil.
func NewPublisher(cfg Config, observer Observer) (*Publisher, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cl, err := newClient(cfg, kgo.RequiredAcks(kgo.AllISRAcks()))
	if err != nil {
		return nil, fmt.Errorf("dmpfkafka: producer: %w", err)
	}
	p, err := newPublisher(cfg, cl, observer)
	if err != nil {
		cl.Close()
		return nil, err
	}
	return p, nil
}

func newPublisher(cfg Config, cl client, observer Observer) (*Publisher, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	call, err := compose.Build(composition(cfg, "publish"))
	if err != nil {
		return nil, err
	}
	return &Publisher{cfg: cfg, client: cl, call: call, observer: observer, ops: operations(cfg, "publish", channel.Kafka)}, nil
}

// operations is one resilience.Operation per catalogued channel of the
// transport, built once: the publish path then allocates nothing for it.
func operations(cfg Config, prefix string, transport channel.Transport) map[string]resilience.Operation {
	cc := composition(cfg, prefix)
	ops := make(map[string]resilience.Operation, len(cfg.Catalog))
	for _, ch := range cfg.Catalog {
		if ch.Transport == transport {
			ops[ch.Name] = compose.Operation(cc, prefix+" "+ch.Name, true)
		}
	}
	return ops
}

// Publish routes the message by logical destination (BLK-04): the catalogue
// gives the topic, the envelope gives the key, and the bytes go untouched.
func (p *Publisher) Publish(ctx context.Context, destination string, message []byte) error {
	ch, err := p.cfg.Channel(destination)
	if err != nil {
		return err
	}
	env, err := envelope.Unmarshal(message)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidEnvelope, destination)
	}

	record := &kgo.Record{
		Topic:   ch.Address,
		Key:     []byte(env.PartitionKey),
		Value:   message,
		Headers: []kgo.RecordHeader{{Key: HeaderPublishedAt, Value: []byte(p.cfg.Clock.Now().UTC().Format(time.RFC3339Nano))}},
	}
	if p.observer != nil {
		p.observer(view(record))
	}

	return p.call(ctx, p.ops[ch.Name], func(ctx context.Context) error {
		return p.client.ProduceSync(ctx, record).FirstErr()
	})
}

// Close releases the producer, flushing what is buffered.
func (p *Publisher) Close() { p.client.Close() }

// composition is what dmpf-transport/compose needs from this provider, with the
// Kafka error code as the failure category and the producer classifier.
func composition(cfg Config, prefix string) compose.Config {
	return compose.Config{
		Sheet:       cfg.Sheet,
		Service:     cfg.Service,
		SpanPrefix:  "dmpf.kafka." + prefix + " ",
		Clock:       cfg.Clock,
		Tracer:      cfg.Tracer,
		Instruments: cfg.Instruments,
		Logger:      cfg.Logger,
		Rand:        cfg.Rand,
		Category:    categoryOf,
		Classifier:  Classifier,
	}
}
