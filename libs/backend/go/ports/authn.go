// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-07 §4), dentro do limite de 3 linhas.

package ports

import (
	"context"
	"errors"
)

// Credential is the material presented at this edge, opaque to the port: the
// scheme names how it arrived and the value carries it. Presenting it is the
// first of the three conditions of IDN-01, never the whole of them.
type Credential struct {
	Scheme string
	Value  string
}

// Presented reports whether any material arrived. It answers the first
// condition of IDN-01 alone and never stands for verification.
func (c Credential) Presented() bool { return c.Value != "" }

// Identity is what verification resolved: the subject, the tenant under the
// predicate of ENV-12, and the effective permissions (IDN-10). Absent tenant is
// nil, never a synthetic value (IDN-20).
type Identity struct {
	Subject     SubjectID
	Tenant      *TenantID
	Permissions []Permission
}

// ErrCredentialAbsent reports that no credential was presented. It is the
// failure of the first condition of IDN-01.
var ErrCredentialAbsent = errors.New("ports: no credential presented")

// ErrCredentialRejected reports that the authority refused the credential. It
// is the failure of the second condition of IDN-01.
var ErrCredentialRejected = errors.New("ports: credential rejected by the authority")

// ErrSubjectUnresolved reports that verification succeeded without yielding a
// subject. It is the failure of the third condition of IDN-01, which no other
// condition implies.
var ErrSubjectUnresolved = errors.New("ports: verification resolved no subject")

// Authenticator verifies a presented credential against the authentication
// authority and resolves the subject (IDN-01). Reaching the authority is
// io.network, so the port declares it and a provider or app realizes it.
type Authenticator interface {
	Authenticate(ctx context.Context, credential Credential) (Identity, error)
}
