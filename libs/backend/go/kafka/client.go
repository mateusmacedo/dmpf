// comment-discipline-ok-file: arquivo de contrato interno; o godoc cita a regra de FND-06 (TRP-28, TRP-29) que o símbolo realiza, dentro do limite de 3 linhas.

package kafka

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

// client is the slice of *kgo.Client this provider uses, declared here so a
// test substitutes it. CommitRecords is the commit gesture because it is
// synchronous and commits one past the record (TRP-29) without kmsg types.
type client interface {
	PollRecords(ctx context.Context, maxPollRecords int) kgo.Fetches
	CommitRecords(ctx context.Context, rs ...*kgo.Record) error
	PauseFetchPartitions(topicPartitions map[string][]int32) map[string][]int32
	ResumeFetchPartitions(topicPartitions map[string][]int32)
	AllowRebalance()
	ProduceSync(ctx context.Context, rs ...*kgo.Record) kgo.ProduceResults
	Close()
}

var _ client = (*kgo.Client)(nil)

// newClient opens the real client: seed brokers, TLS when configured, plus the
// options of the producer or of the consumer group that calls it.
func newClient(cfg Config, opts ...kgo.Opt) (*kgo.Client, error) {
	options := []kgo.Opt{kgo.SeedBrokers(cfg.Brokers...)}
	if cfg.TLS != nil {
		options = append(options, kgo.DialTLSConfig(cfg.TLS))
	} else {
		cfg.logger().Warn("kafka: brokers without TLS by explicit development-only opt-out")
	}
	if cfg.SASL != nil {
		mechanism, err := cfg.SASL.mechanism()
		if err != nil {
			return nil, err
		}
		options = append(options, kgo.SASL(mechanism))
	}
	options = append(options, opts...)
	return kgo.NewClient(options...)
}
