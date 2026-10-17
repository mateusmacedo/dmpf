package authn

import (
	"context"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// SessionRef carries only verified claims: the checker is consulted after
// signature, issuer, audience, expiry and subject (IDN-10). ID is empty when
// the token has no sid.
type SessionRef struct {
	Subject  ports.SubjectID
	ID       string
	IssuedAt time.Time
}

// RevocationKind names the entry that denied a token; the zero value means not
// revoked. When both entries deny, session takes precedence.
type RevocationKind string

const (
	RevocationSession RevocationKind = "session"
	RevocationSubject RevocationKind = "subject"
)

type RevocationChecker interface {
	Revoked(ctx context.Context, ref SessionRef) (RevocationKind, error)
}

// Revocation is one entry to record. A subject entry denies tokens whose iat is
// not after OccurredAt; ExpiresAt bounds how long the entry is kept.
type Revocation struct {
	EventID    string
	Kind       RevocationKind
	Subject    ports.SubjectID
	SessionID  string
	OccurredAt time.Time
	ExpiresAt  time.Time
}

// RevocationStore keeps the greatest OccurredAt seen per key, so redelivery and
// reordering never shorten a revocation. Revoke reports whether the entry
// changed.
type RevocationStore interface {
	RevocationChecker
	Revoke(ctx context.Context, r Revocation) (bool, error)
}
