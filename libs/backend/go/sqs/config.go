// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (ASY-01, TRP-07, SQS-04) ou FND-08 (RES-21) que o símbolo realiza, dentro do limite de 3 linhas.

package sqs

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
)

// Config is the SQS/SNS side of one process: the resolved AWS configuration,
// an optional endpoint, the channel catalogue every queue URL and topic ARN
// live in (TRP-07, ASY-01), the sheet of RES-21, and the decorators' outputs.
type Config struct {
	AWS                        aws.Config
	Endpoint                   string
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

// Validate refuses a configuration without region, clock or catalogue, a
// plaintext endpoint without the development-only opt-out, an invalid
// catalogue, or a sheet with a blank.
func (c Config) Validate() error {
	switch {
	case c.AWS.Region == "":
		return fmt.Errorf("%w: aws region", ErrIncompleteConfig)
	case c.Clock == nil:
		return fmt.Errorf("%w: clock", ErrIncompleteConfig)
	case len(c.Catalog) == 0:
		return fmt.Errorf("%w: catalogue declares no channel (ASY-01)", ErrIncompleteConfig)
	case c.Endpoint != "" && !strings.HasPrefix(c.Endpoint, "https://") && !c.InsecureForDevelopmentOnly:
		return ErrTLSRequired
	}
	if err := c.Catalog.Validate(); err != nil {
		return err
	}
	return c.Sheet.Validate()
}

// Channel resolves a logical destination to its SQS or SNS → SQS channel and
// checks the queue type against the promise of order (SQS-04): FIFO orders by
// group, standard by nothing.
func (c Config) Channel(destination string) (channel.Channel, error) {
	ch, err := c.Catalog.Resolve(destination)
	if err != nil {
		return channel.Channel{}, err
	}
	if ch.Transport != channel.SQS && ch.Transport != channel.SNSSQS {
		return channel.Channel{}, fmt.Errorf("%w: %s is %s", ErrNotSQSChannel, ch.Name, ch.Transport)
	}
	if IsFIFO(ch) != (ch.Ordering.Unit == channel.Group) {
		return channel.Channel{}, fmt.Errorf("%w: %s: fifo=%v, unit=%s", ErrOrderingMismatch, ch.Name, IsFIFO(ch), ch.Ordering.Unit)
	}
	return ch, nil
}

// IsFIFO reads the queue type from the address: SQS names FIFO queues with the
// .fifo suffix, and so does an SNS FIFO topic.
func IsFIFO(ch channel.Channel) bool {
	const suffix = ".fifo"
	return len(ch.Address) > len(suffix) && ch.Address[len(ch.Address)-len(suffix):] == suffix
}

func (c Config) logger() *slog.Logger {
	if c.Logger == nil {
		return slog.Default()
	}
	return c.Logger
}
