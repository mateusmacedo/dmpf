package authn

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Verifier realizes ports.Authenticator against an OIDC authorization server.
// Discovery, the JWKS and its rotation are the library's; what stays here is
// reading the claims the operator declared (IDN-01).
type Verifier struct {
	tokens           *oidc.IDTokenVerifier
	tenantClaim      string
	permissionClaims []string
	logger           *slog.Logger
}

// NewVerifier discovers the issuer at start, so a process that cannot reach the
// authority fails to start instead of serving requests it could not
// authenticate.
//
// WHY: Keycloak only puts the resource server in `aud` when the client has an
// audience mapper; without one it issues `aud: ["account"]` and every token is
// refused here. That is configuration of the realm, not of this edge.
func NewVerifier(ctx context.Context, cfg Config) (*Verifier, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	discovery, cancel := context.WithTimeout(ctx, cfg.DiscoveryTimeout)
	defer cancel()

	client := &http.Client{Timeout: cfg.DiscoveryTimeout, Transport: otelhttp.NewTransport(http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method }))}
	provider, err := oidc.NewProvider(oidc.ClientContext(discovery, client), cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discovering %s: %w", cfg.Issuer, err)
	}

	return &Verifier{
		tokens:           provider.Verifier(&oidc.Config{ClientID: cfg.Audience}),
		tenantClaim:      cfg.TenantClaim,
		permissionClaims: cfg.PermissionClaims,
		logger:           logging.NewLogger(cfg.LoggerProvider, reflect.TypeFor[Verifier]().PkgPath()),
	}, nil
}

func (v *Verifier) Authenticate(ctx context.Context, credential ports.Credential) (ports.Identity, error) {
	if !credential.Presented() {
		return ports.Identity{}, ports.ErrCredentialAbsent
	}
	if !strings.EqualFold(credential.Scheme, bearerScheme) {
		v.refused(ctx, refusalMalformed)
		return ports.Identity{}, fmt.Errorf("%w: unsupported scheme %q", ports.ErrCredentialRejected, credential.Scheme)
	}

	token, err := v.tokens.Verify(ctx, credential.Value)
	if err != nil {
		if reason, refused := refusalOf(err); refused {
			v.refused(ctx, reason)
		} else {
			v.keysUnavailable(ctx)
		}
		return ports.Identity{}, fmt.Errorf("%w: %w", ports.ErrCredentialRejected, err)
	}
	if token.Subject == "" {
		v.refused(ctx, refusalMalformed)
		return ports.Identity{}, ports.ErrSubjectUnresolved
	}

	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		v.refused(ctx, refusalMalformed)
		return ports.Identity{}, fmt.Errorf("%w: claims are unreadable: %w", ports.ErrCredentialRejected, err)
	}

	identity := ports.Identity{
		Subject:     ports.SubjectID(token.Subject),
		Permissions: permissionsFrom(claims, v.permissionClaims),
	}
	if tenant, ok := stringClaim(claims, v.tenantClaim); ok {
		resolved := ports.TenantID(tenant)
		identity.Tenant = &resolved
	}
	return identity, nil
}

const (
	refusalExpired          = "token_expired"
	refusalAudienceMismatch = "audience_mismatch"
	refusalIssuerMismatch   = "issuer_mismatch"
	refusalSignatureInvalid = "signature_invalid"
	refusalMalformed        = "malformed"
)

const (
	keyRefusalReason            = "dmpf.auth.refusal_reason"
	categoryUnauthenticated     = "Unauthenticated"
	categoryTransientDependency = "TransientDependency"
)

func (v *Verifier) refused(ctx context.Context, reason string) {
	v.logger.WarnContext(ctx, "auth: token rejected",
		slog.String(string(semconv.ErrorTypeKey), categoryUnauthenticated), slog.String(keyRefusalReason, reason))
}

func (v *Verifier) keysUnavailable(ctx context.Context) {
	v.logger.Log(ctx, logging.Severity(logging.Client, ports.OutcomeFailed), "auth: signing keys unavailable",
		slog.String(string(semconv.ErrorTypeKey), categoryTransientDependency))
}

// go-oidc v3.21.0 types only the expiry; the other refusals, and the JWKS fetch
// that fails before any signature is checked, are told apart by the text it
// builds in oidc/verify.go:244,254,336 and oidc/jwks.go:178.
func refusalOf(err error) (reason string, refused bool) {
	var expired *oidc.TokenExpiredError
	message := err.Error()
	switch {
	case errors.As(err, &expired):
		return refusalExpired, true
	case strings.HasPrefix(message, "oidc: expected audience"):
		return refusalAudienceMismatch, true
	case strings.HasPrefix(message, "oidc: id token issued by a different provider"):
		return refusalIssuerMismatch, true
	case strings.HasPrefix(message, "failed to verify signature: fetching keys"):
		return "", false
	case strings.HasPrefix(message, "failed to verify signature"):
		return refusalSignatureInvalid, true
	default:
		return refusalMalformed, true
	}
}
