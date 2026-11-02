// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra (DAT-08, DAT-14, IDN-17, RES-21) ou a exigência da SPEC-CWBA4BPV que o símbolo realiza, dentro do limite de 3 linhas.

package s3

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Config is one store: the resolved AWS configuration, the bucket, an optional
// endpoint, what the store declares (scope, encryption and ceiling, IDN-17,
// DAT-08, DAT-14), the sheet of RES-21 and the decorators' outputs.
type Config struct {
	AWS                        aws.Config
	Bucket                     string
	Endpoint                   string
	InsecureForDevelopmentOnly bool
	UsePathStyle               bool
	Declaration                ports.ObjectStoreDeclaration
	Clock                      clock.Clock
	Sheet                      resilience.Sheet
	Tracer                     trace.Tracer
	Instruments                *metrics.Instruments
	LoggerProvider             log.LoggerProvider
	Rand                       func() float64
}

// Validate refuses a missing region, bucket or clock, path-style without an
// endpoint, plaintext without the development-only opt-out, an incomplete
// declaration, a KMS key not declared by its ARN (DAT-08) or a sheet blank.
func (c Config) Validate() error {
	switch {
	case c.AWS.Region == "":
		return fmt.Errorf("%w: aws region", ErrIncompleteConfig)
	case c.Bucket == "":
		return fmt.Errorf("%w: bucket", ErrIncompleteConfig)
	case c.Clock == nil:
		return fmt.Errorf("%w: clock", ErrIncompleteConfig)
	case c.UsePathStyle && c.Endpoint == "":
		return fmt.Errorf("%w: path-style addressing needs an explicit endpoint", ErrIncompleteConfig)
	case c.Endpoint != "" && !strings.HasPrefix(c.Endpoint, "https://") && !c.InsecureForDevelopmentOnly:
		return ErrTLSRequired
	}
	if err := c.Declaration.Validate(); err != nil {
		return err
	}
	if e := c.Declaration.Encryption; strings.HasPrefix(e.Algorithm, "aws:kms") && !strings.HasPrefix(e.KeyRef, "arn:") {
		return fmt.Errorf("%w: a KMS key is declared by its ARN, the form S3 reports it in (DAT-08)", ErrIncompleteConfig)
	}
	return c.Sheet.Validate()
}

func (c Config) logger() *slog.Logger {
	return logging.NewLogger(c.LoggerProvider, reflect.TypeFor[Config]().PkgPath())
}
