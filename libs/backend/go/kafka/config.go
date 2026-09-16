// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (ASY-01, TRP-07, KFK-01) ou FND-08 (RES-21) que o símbolo realiza, dentro do limite de 3 linhas.

package kafka

import (
	"crypto/tls"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

// Config is the Kafka side of one process: brokers, transport security, the
// channel catalogue every topic name lives in (TRP-07, ASY-01), the sheet of
// RES-21, and what the decorators record through.
type Config struct {
	Brokers                    []string
	TLS                        *tls.Config
	InsecureForDevelopmentOnly bool
	Catalog                    channel.Catalog
	Sheet                      resilience.Sheet
	Service                    string
	Clock                      clock.Clock
	Tracer                     trace.Tracer
	Instruments                *metrics.Instruments
	Logger                     *slog.Logger
	Rand                       func() float64
}

// Validate refuses a configuration without brokers, clock or catalogue, one
// without transport security and without the opt-out, a TLS below the floor,
// an invalid catalogue, or a sheet with a blank.
func (c Config) Validate() error {
	switch {
	case len(c.Brokers) == 0:
		return fmt.Errorf("%w: brokers", ErrIncompleteConfig)
	case c.Clock == nil:
		return fmt.Errorf("%w: clock", ErrIncompleteConfig)
	case len(c.Catalog) == 0:
		return fmt.Errorf("%w: catalogue declares no channel (ASY-01)", ErrIncompleteConfig)
	case c.TLS == nil && !c.InsecureForDevelopmentOnly:
		return ErrTLSRequired
	case c.TLS != nil && (c.TLS.InsecureSkipVerify || (c.TLS.MinVersion != 0 && c.TLS.MinVersion < tls.VersionTLS12)):
		return ErrTLSTooWeak
	}
	if err := c.Catalog.Validate(); err != nil {
		return err
	}
	return c.Sheet.Validate()
}

// Channel resolves a logical destination to its Kafka channel: unknown
// destinations and channels bound to another transport are refused.
func (c Config) Channel(destination string) (channel.Channel, error) {
	ch, err := c.Catalog.Resolve(destination)
	if err != nil {
		return channel.Channel{}, err
	}
	if ch.Transport != channel.Kafka {
		return channel.Channel{}, fmt.Errorf("%w: %s is %s", ErrNotKafkaChannel, ch.Name, ch.Transport)
	}
	return ch, nil
}

func (c Config) logger() *slog.Logger {
	if c.Logger == nil {
		return slog.Default()
	}
	return c.Logger
}
