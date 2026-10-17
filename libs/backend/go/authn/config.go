package authn

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
)

var (
	// ErrMissingVariable is a partially configured verifier: the issuer is set
	// and something the verification depends on is not.
	ErrMissingVariable = errors.New("auth: required variable is not set")

	// ErrVerifierNotDeclared refuses a start that resolves identity by no means
	// at all: without an issuer, the development mock has to say so explicitly.
	ErrVerifierNotDeclared = errors.New("auth: no issuer configured and AUTH_DEV_MOCK is not set")

	// ErrMockWithVerifier refuses a start that declares both: one process
	// resolves identity one way, and the ambiguity would decide itself.
	ErrMockWithVerifier = errors.New("auth: AUTH_DEV_MOCK cannot be set alongside a configured issuer")
)

const (
	envIssuer            = "OIDC_ISSUER"
	envAudience          = "OIDC_AUDIENCE"
	envTenantClaim       = "OIDC_TENANT_CLAIM"
	envPermissionClaims  = "OIDC_PERMISSION_CLAIMS"
	envDiscoveryTimeout  = "OIDC_DISCOVERY_TIMEOUT_SECONDS"
	envDevMock           = "AUTH_DEV_MOCK"
	envRevocationTimeout = "OIDC_REVOCATION_TIMEOUT_MS"
)

const (
	defaultDiscoveryTimeoutSeconds       = 10
	defaultRevocationTimeoutMilliseconds = 250
)

// Config is how the operator points the edge at an authorization server. Every
// claim is a path, so an issuer that nests or namespaces them needs no code
// change (IDN-16 stays a declaration, not an implementation detail).
type Config struct {
	Issuer           string
	Audience         string
	TenantClaim      string
	PermissionClaims []string
	DiscoveryTimeout time.Duration
	DevMock          bool
	LoggerProvider   log.LoggerProvider

	Revocations       RevocationChecker
	RevocationTimeout time.Duration
	MeterProvider     metric.MeterProvider
}

// Defaults reads a Keycloak access token: scope as a space-separated string and
// realm roles nested under realm_access.
func Defaults() Config {
	return Config{
		PermissionClaims:  []string{"scope", "realm_access.roles"},
		DiscoveryTimeout:  defaultDiscoveryTimeoutSeconds * time.Second,
		RevocationTimeout: defaultRevocationTimeoutMilliseconds * time.Millisecond,
	}
}

// FromEnv reads and validates in one step. A caller that aggregates several
// configurations and wants to control the order of refusals reads with ReadEnv
// and validates later.
func FromEnv(lookup func(string) string) (Config, error) {
	cfg, err := ReadEnv(lookup)
	if err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// ReadEnv parses without deciding whether the result is a usable start.
func ReadEnv(lookup func(string) string) (Config, error) {
	cfg := Defaults()

	cfg.Issuer = lookup(envIssuer)
	cfg.Audience = lookup(envAudience)
	cfg.TenantClaim = lookup(envTenantClaim)
	if declared := envconfig.SplitList(lookup(envPermissionClaims)); len(declared) > 0 {
		cfg.PermissionClaims = declared
	}

	var err error
	if declared := lookup(envDiscoveryTimeout); declared != "" {
		if cfg.DiscoveryTimeout, err = parseSeconds(envDiscoveryTimeout, declared, 1); err != nil {
			return Config{}, err
		}
	}
	if declared := lookup(envRevocationTimeout); declared != "" {
		if cfg.RevocationTimeout, err = parseMilliseconds(envRevocationTimeout, declared); err != nil {
			return Config{}, err
		}
	}

	if cfg.DevMock, err = envconfig.ParseBool(envDevMock, lookup(envDevMock)); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("issuer", c.Issuer),
		slog.String("audience", c.Audience),
		slog.String("tenant_claim", c.TenantClaim),
		slog.String("permission_claims", strings.Join(c.PermissionClaims, ",")),
		slog.String("discovery_timeout", c.DiscoveryTimeout.String()),
		slog.Bool("dev_mock", c.DevMock),
		slog.String("revocation", c.revocationState()),
	)
}

func (c Config) revocationState() string {
	if c.Revocations == nil {
		return "disabled"
	}
	return "enabled"
}

// Validate refuses a start whose identity the edge could not resolve: no means
// declared at all, two means declared at once, or a verifier missing what the
// verification depends on.
func (c Config) Validate() error {
	switch {
	case c.DevMock && c.Issuer != "":
		return ErrMockWithVerifier
	case c.DevMock:
		return nil
	case c.Issuer == "":
		return fmt.Errorf("%w: set %s or %s=true", ErrVerifierNotDeclared, envIssuer, envDevMock)
	case c.Audience == "":
		return fmt.Errorf("%w: %s", ErrMissingVariable, envAudience)
	case c.TenantClaim == "":
		return fmt.Errorf("%w: %s", ErrMissingVariable, envTenantClaim)
	case c.Revocations != nil && c.RevocationTimeout <= 0:
		return fmt.Errorf("%w: %s must be positive when revocation is enabled, got %v",
			envconfig.ErrInvalidVariable, envRevocationTimeout, c.RevocationTimeout)
	}
	return nil
}
