// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (GRP-08..16) ou FND-08 (RES-21) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfgrpc

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/deadline"
)

var (
	// ErrTLSRequired is a client or server with neither a TLS configuration nor
	// the explicit development-only opt-out: in production the channel is
	// encrypted (GRP-15).
	ErrTLSRequired = errors.New("dmpfgrpc: TLS is required outside development (GRP-15)")

	// ErrTLSTooWeak is a TLS configuration that skips peer verification or
	// admits a protocol version below 1.2 (GRP-15).
	ErrTLSTooWeak = errors.New("dmpfgrpc: TLS must verify the peer and require at least TLS 1.2 (GRP-15)")

	// ErrMethodNotDeclared is a call to a method with no declared policy: there
	// is no default deadline and no default retry (GRP-04, GRP-16).
	ErrMethodNotDeclared = errors.New("dmpfgrpc: method declares no policy (GRP-16)")

	// ErrIncompleteConfig is a configuration without clock or without methods.
	ErrIncompleteConfig = errors.New("dmpfgrpc: configuration is incomplete")
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

// Config is the client-side configuration of one dependency: transport
// security, the sheet of RES-21, the policy per full method name, and what the
// decorators record through; Service labels the series of MET-08 to MET-10.
type Config struct {
	TLS                        *tls.Config
	InsecureForDevelopmentOnly bool
	Sheet                      resilience.Sheet
	Methods                    map[string]MethodPolicy
	HealthServiceName          string
	Service                    string
	Clock                      clock.Clock
	Tracer                     trace.Tracer
	Instruments                *metrics.Instruments
	Logger                     *slog.Logger
	Rand                       func() float64
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
			return fmt.Errorf("dmpfgrpc: %s: %w", method, err)
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

func (c Config) logger() *slog.Logger {
	if c.Logger == nil {
		return slog.Default()
	}
	return c.Logger
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

// transportCredentials is TLS when configured, and the insecure credentials
// only under the explicit opt-out, which is logged so it never passes unseen.
func transportCredentials(c Config) credentials.TransportCredentials {
	if c.TLS != nil {
		return credentials.NewTLS(c.TLS)
	}
	c.logger().Warn("dmpfgrpc: transport without TLS by explicit development-only opt-out (GRP-15)",
		slog.String("dependency", c.Sheet.Dependency))
	return insecure.NewCredentials()
}
