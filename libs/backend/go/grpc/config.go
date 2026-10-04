// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (GRP-08..16) ou FND-08 (RES-21) que o símbolo realiza, dentro do limite de 3 linhas.

package grpc

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

var (
	// ErrTLSRequired is a client or server with neither a TLS configuration nor
	// the explicit development-only opt-out: in production the channel is
	// encrypted (GRP-15).
	ErrTLSRequired = errors.New("grpc: TLS is required outside development (GRP-15)")

	// ErrTLSTooWeak is a TLS configuration that skips peer verification or
	// admits a protocol version below 1.2 (GRP-15).
	ErrTLSTooWeak = errors.New("grpc: TLS must verify the peer and require at least TLS 1.2 (GRP-15)")

	// ErrMethodNotDeclared is a call to a method with no declared policy: there
	// is no default deadline and no default retry (GRP-04, GRP-16).
	ErrMethodNotDeclared = errors.New("grpc: method declares no policy (GRP-16)")

	// ErrIncompleteConfig is a configuration without clock or without methods.
	ErrIncompleteConfig = errors.New("grpc: configuration is incomplete")
)

// MethodPolicy is what one method declares: its deadline budget, whether it
// may be repeated, and which status codes count as transient for it (GRP-08,
// GRP-09, GRP-16). Attempts and backoff come from the Sheet, never per method.
type MethodPolicy struct {
	Budget         deadline.Budget
	Idempotent     bool
	RetryableCodes []codes.Code
}

// Validate refuses a policy whose budget the deadline package would refuse.
func (p MethodPolicy) Validate() error {
	return p.Budget.Validate()
}

func (p MethodPolicy) LogValue() slog.Value {
	retryable := make([]string, len(p.RetryableCodes))
	for i, code := range p.RetryableCodes {
		retryable[i] = code.String()
	}
	return slog.GroupValue(
		slog.String("limit", p.Budget.Limit.String()),
		slog.String("slack", p.Budget.Slack.String()),
		slog.String("estimated_duration", p.Budget.EstimatedDuration.String()),
		slog.Bool("idempotent", p.Idempotent),
		slog.String("retryable_codes", strings.Join(retryable, ",")),
	)
}

// Config is the client-side configuration of one dependency: transport
// security, the sheet of RES-21, the policy per full method name, and what the
// decorators record through.
type Config struct {
	TLS                        *tls.Config
	InsecureForDevelopmentOnly bool
	Sheet                      resilience.Sheet
	Methods                    map[string]MethodPolicy
	HealthServiceName          string
	Clock                      clock.Clock
	Tracer                     trace.Tracer
	TracerProvider             trace.TracerProvider
	MeterProvider              metric.MeterProvider
	Propagator                 propagation.TextMapPropagator
	Instruments                *metrics.Instruments
	LoggerProvider             log.LoggerProvider
	Rand                       func() float64
}

func (c Config) LogValue() slog.Value {
	methods := make([]slog.Attr, 0, len(c.Methods))
	for _, method := range slices.Sorted(maps.Keys(c.Methods)) {
		methods = append(methods, slog.Any(method, c.Methods[method]))
	}
	return slog.GroupValue(
		slog.Bool("tls", c.TLS != nil),
		slog.Bool("insecure_for_development_only", c.InsecureForDevelopmentOnly),
		slog.Any("sheet", c.Sheet),
		slog.Attr{Key: "methods", Value: slog.GroupValue(methods...)},
	)
}

// Validate refuses a configuration a Dial could not honour: no transport
// security and no opt-out, a TLS below the floor of GRP-15, a sheet with a
// blank, no clock, no method, or a method whose budget is invalid.
func (c Config) Validate() error {
	if err := validateTLS(c.TLS, c.InsecureForDevelopmentOnly); err != nil {
		return err
	}
	if c.Clock == nil {
		return fmt.Errorf("%w: clock", ErrIncompleteConfig)
	}
	if len(c.Methods) == 0 {
		return fmt.Errorf("%w: no method declares a policy (GRP-16)", ErrIncompleteConfig)
	}
	if err := c.Sheet.Validate(); err != nil {
		return err
	}
	for method, policy := range c.Methods {
		if method == "" {
			return fmt.Errorf("%w: empty method name", ErrIncompleteConfig)
		}
		if err := policy.Validate(); err != nil {
			return fmt.Errorf("grpc: %s: %w", method, err)
		}
	}
	return nil
}

// Policy returns the declared policy of a full method name, or
// ErrMethodNotDeclared.
func (c Config) Policy(method string) (MethodPolicy, error) {
	policy, declared := c.Methods[method]
	if !declared {
		return MethodPolicy{}, fmt.Errorf("%w: %q", ErrMethodNotDeclared, method)
	}
	return policy, nil
}

func (c Config) logger() *slog.Logger { return loggerOf(c.LoggerProvider) }

func loggerOf(provider log.LoggerProvider) *slog.Logger {
	return logging.NewLogger(provider, reflect.TypeFor[Config]().PkgPath())
}

// validateTLS is the gate of GRP-15 shared by client and server: TLS or the
// explicit opt-out, and a TLS that verifies the peer at 1.2 or above.
func validateTLS(cfg *tls.Config, insecureOptOut bool) error {
	switch {
	case cfg == nil && !insecureOptOut:
		return ErrTLSRequired
	case cfg != nil && (cfg.InsecureSkipVerify || (cfg.MinVersion != 0 && cfg.MinVersion < tls.VersionTLS12)):
		return ErrTLSTooWeak
	}
	return nil
}

var insecureClientWarning = new(sync.Once)

// transportCredentials is TLS when configured, and the insecure credentials
// only under the explicit opt-out, logged once per process so it never passes unseen.
func transportCredentials(c Config) credentials.TransportCredentials {
	if c.TLS != nil {
		return credentials.NewTLS(c.TLS)
	}
	insecureClientWarning.Do(func() {
		c.logger().Warn("grpc: transport without TLS by explicit development-only opt-out (GRP-15)")
	})
	return insecure.NewCredentials()
}
