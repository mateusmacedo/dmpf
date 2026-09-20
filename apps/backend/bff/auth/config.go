package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
)

var (
	// ErrMissingVariable is a partially configured verifier: the issuer is set
	// and something the verification depends on is not.
	ErrMissingVariable = errors.New("auth: required variable is not set")

	// ErrVerifierNotDeclared refuses a start that resolves identity by no means
	// at all: without an issuer, the development mock has to say so explicitly.
	ErrVerifierNotDeclared = errors.New("auth: no issuer configured and DMPF_AUTH_DEV_MOCK is not set")

	// ErrMockWithVerifier refuses a start that declares both: one process
	// resolves identity one way, and the ambiguity would decide itself.
	ErrMockWithVerifier = errors.New("auth: DMPF_AUTH_DEV_MOCK cannot be set alongside a configured issuer")
)

const (
	envIssuer           = "DMPF_OIDC_ISSUER"
	envAudience         = "DMPF_OIDC_AUDIENCE"
	envTenantClaim      = "DMPF_OIDC_TENANT_CLAIM"
	envPermissionClaims = "DMPF_OIDC_PERMISSION_CLAIMS"
	envDiscoveryTimeout = "DMPF_OIDC_DISCOVERY_TIMEOUT_SECONDS"
	envDevMock          = "DMPF_AUTH_DEV_MOCK"
)

const defaultDiscoveryTimeoutSeconds = 10

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
}

// Defaults reads a Keycloak access token: scope as a space-separated string and
// realm roles nested under realm_access.
func Defaults() Config {
	return Config{
		PermissionClaims: []string{"scope", "realm_access.roles"},
		DiscoveryTimeout: defaultDiscoveryTimeoutSeconds * time.Second,
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

	seconds, err := envconfig.ParsePositive(envDiscoveryTimeout, lookup(envDiscoveryTimeout), defaultDiscoveryTimeoutSeconds)
	if err != nil {
		return Config{}, err
	}
	cfg.DiscoveryTimeout = time.Duration(seconds) * time.Second

	if cfg.DevMock, err = envconfig.ParseBool(envDevMock, lookup(envDevMock)); err != nil {
		return Config{}, err
	}
	return cfg, nil
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
	}
	return nil
}
