package authn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	authorizationHeader = "Authorization"
	bearerScheme        = "Bearer"
)

// CredentialFrom reads what the caller presented, and nothing more: the header
// is the material, never the subject nor the tenant, which only verification
// resolves (CTX-06).
func CredentialFrom(r *http.Request) ports.Credential {
	raw := strings.TrimSpace(r.Header.Get(authorizationHeader))
	if raw == "" {
		return ports.Credential{}
	}

	scheme, value, found := strings.Cut(raw, " ")
	if !found {
		return ports.Credential{Scheme: raw}
	}
	return ports.Credential{Scheme: scheme, Value: strings.TrimSpace(value)}
}

// DevAuthenticator resolves identity from the credential itself, which means
// any caller can forge any subject and tenant. It exists for development and
// for the black-box end-to-end suite, and the start refuses it unless
// DMPF_AUTH_DEV_MOCK declares it (IDN-01 is satisfied by no part of this).
type DevAuthenticator struct{}

type devIdentity struct {
	Subject     string   `json:"sub"`
	Tenant      *string  `json:"tenant"`
	Permissions []string `json:"permissions"`
}

func (DevAuthenticator) Authenticate(_ context.Context, credential ports.Credential) (ports.Identity, error) {
	if !credential.Presented() {
		return ports.Identity{}, ports.ErrCredentialAbsent
	}
	if !strings.EqualFold(credential.Scheme, bearerScheme) {
		return ports.Identity{}, fmt.Errorf("%w: unsupported scheme %q", ports.ErrCredentialRejected, credential.Scheme)
	}

	var declared devIdentity
	if err := json.Unmarshal([]byte(credential.Value), &declared); err != nil {
		return ports.Identity{}, fmt.Errorf("%w: the development credential is not a declaration", ports.ErrCredentialRejected)
	}
	if declared.Subject == "" {
		return ports.Identity{}, ports.ErrSubjectUnresolved
	}

	identity := ports.Identity{
		Subject:     ports.SubjectID(declared.Subject),
		Permissions: make([]ports.Permission, 0, len(declared.Permissions)),
	}
	if declared.Tenant != nil && *declared.Tenant != "" {
		tenant := ports.TenantID(*declared.Tenant)
		identity.Tenant = &tenant
	}
	for _, granted := range declared.Permissions {
		identity.Permissions = append(identity.Permissions, ports.Permission(granted))
	}
	slices.Sort(identity.Permissions)
	return identity, nil
}
