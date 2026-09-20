package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Verifier realizes ports.Authenticator against an OIDC authorization server.
// Discovery, the JWKS and its rotation are the library's; what stays here is
// reading the claims the operator declared (IDN-01).
type Verifier struct {
	tokens           *oidc.IDTokenVerifier
	tenantClaim      string
	permissionClaims []string
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

	client := &http.Client{Timeout: cfg.DiscoveryTimeout}
	provider, err := oidc.NewProvider(oidc.ClientContext(discovery, client), cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discovering %s: %w", cfg.Issuer, err)
	}

	return &Verifier{
		tokens:           provider.Verifier(&oidc.Config{ClientID: cfg.Audience}),
		tenantClaim:      cfg.TenantClaim,
		permissionClaims: cfg.PermissionClaims,
	}, nil
}

func (v *Verifier) Authenticate(ctx context.Context, credential ports.Credential) (ports.Identity, error) {
	if !credential.Presented() {
		return ports.Identity{}, ports.ErrCredentialAbsent
	}
	if !strings.EqualFold(credential.Scheme, bearerScheme) {
		return ports.Identity{}, fmt.Errorf("%w: unsupported scheme %q", ports.ErrCredentialRejected, credential.Scheme)
	}

	token, err := v.tokens.Verify(ctx, credential.Value)
	if err != nil {
		return ports.Identity{}, fmt.Errorf("%w: %w", ports.ErrCredentialRejected, err)
	}
	if token.Subject == "" {
		return ports.Identity{}, ports.ErrSubjectUnresolved
	}

	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
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
