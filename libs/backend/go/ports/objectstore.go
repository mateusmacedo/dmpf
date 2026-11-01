// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (DAT-08/11/14-18/25, IDN-13 a IDN-20, SPEC-CWBA4BPV), dentro do limite de 3 linhas.

package ports

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Scope is declared when the store is built, never inferred (IDN-17): a tenant
// store prefixes tenant/<id>/ from the ExecutionContext, a platform store the
// fixed platform/ that no tenant id reaches (IDN-19, IDN-20).
type Scope string

const (
	ScopeTenant   Scope = "tenant"
	ScopePlatform Scope = "platform"
)

type ObjectKey string

// Validate refuses an empty key, a leading slash and a "." or ".." segment, so
// no layer that normalizes paths can carry a key out of its scope prefix. The
// error names the reason, never the key, which may carry personal data (DAT-22).
func (k ObjectKey) Validate() error {
	if k == "" {
		return fmt.Errorf("%w: empty", ErrObjectKeyInvalid)
	}
	if strings.HasPrefix(string(k), "/") {
		return fmt.Errorf("%w: leading slash", ErrObjectKeyInvalid)
	}
	for segment := range strings.SplitSeq(string(k), "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("%w: dot segment", ErrObjectKeyInvalid)
		}
	}
	return nil
}

// VersionID names one immutable version. Every operation after Put takes it,
// because retention and hold protect the version, not the key.
type VersionID string

type ObjectRef struct {
	Key     ObjectKey
	Version VersionID
}

// RetentionMode is COMPLIANCE, which nobody shortens nor downgrades, or
// GOVERNANCE, which the port never bypasses either (DAT-11, DAT-14).
type RetentionMode string

const (
	RetentionCompliance RetentionMode = "COMPLIANCE"
	RetentionGovernance RetentionMode = "GOVERNANCE"
)

func (m RetentionMode) Known() bool {
	return m == RetentionCompliance || m == RetentionGovernance
}

// Retention is always dated: an absent or past Until is unbounded (DAT-17).
// Until has second resolution — S3 keeps milliseconds at most —, so a store
// drops the fraction and a caller comparing what it wrote passes whole seconds.
type Retention struct {
	Mode  RetentionMode
	Until Instant
}

// RetentionSpan is a duration in nanoseconds, kept apart from Instant so that a
// span is never mistaken for a point in time.
type RetentionSpan int64

// Add reports false when the sum leaves the int64 range, instead of wrapping
// into an instant in the past.
func (s RetentionSpan) Add(at Instant) (Instant, bool) {
	sum := at + Instant(s)
	if (s > 0 && sum < at) || (s < 0 && sum > at) {
		return 0, false
	}
	return sum, true
}

// RetentionCeiling is the external ceiling with its declared origin (DAT-14,
// DAT-15): no write, extension or promotion ends past now plus Span.
type RetentionCeiling struct {
	Span   RetentionSpan
	Origin string
}

// Encryption is the at-rest protection the store declares and Head reports from
// the stored version, so DAT-08 is checked on the surface (DAT-03, DAT-08).
type Encryption struct {
	Algorithm string
	KeyRef    string
}

// ObjectStoreDeclaration is what a store is built with. Scope and Encryption are
// required; a store without Ceiling exists and refuses every retention (DAT-17).
type ObjectStoreDeclaration struct {
	Scope      Scope
	Encryption Encryption
	Ceiling    *RetentionCeiling
}

// Validate is pure, so every provider refuses the same incomplete declaration
// with the same sentinel before touching any storage.
func (d ObjectStoreDeclaration) Validate() error {
	switch d.Scope {
	case ScopeTenant, ScopePlatform:
	case "":
		return ErrScopeUndeclared
	default:
		return fmt.Errorf("%w: unknown scope %q", ErrScopeUndeclared, d.Scope)
	}
	if d.Encryption.Algorithm == "" {
		return ErrEncryptionUndeclared
	}
	if d.Ceiling != nil {
		if d.Ceiling.Span <= 0 {
			return fmt.Errorf("%w: ceiling span must be positive", ErrRetentionCeilingUndeclared)
		}
		if d.Ceiling.Origin == "" {
			return fmt.Errorf("%w: ceiling origin is required (DAT-15)", ErrRetentionCeilingUndeclared)
		}
	}
	return nil
}

// PutRequest carries the hex SHA-256 the caller computed; a body that does not
// hash to it is refused unwritten (ErrIntegrity).
type PutRequest struct {
	Key           ObjectKey
	Body          []byte
	ContentSHA256 string
	ContentType   string
	Retention     *Retention
}

// StoredAt is read from the store's clock, the same one that judges retention
// and the ceiling, so it is the "now" those rules are measured from.
type ObjectVersion struct {
	Key           ObjectKey
	Version       VersionID
	ContentSHA256 string
	Size          int64
	StoredAt      Instant
}

func (v ObjectVersion) Ref() ObjectRef { return ObjectRef{Key: v.Key, Version: v.Version} }

// ObjectInfo is the stored state of one version: Retention is nil when none was
// set, and Encryption is read from the version, not from the declaration.
type ObjectInfo struct {
	ObjectVersion
	ContentType string
	Retention   *Retention
	LegalHold   bool
	Encryption  Encryption
}

type Object struct {
	ObjectInfo
	Body []byte
}

// LegalHold has no deadline. Reason is the declared origin of the obligation
// (DAT-15), required to put the hold on.
type LegalHold struct {
	On     bool
	Reason string
}

// HoldReceipt and PurgeReceipt leave through the return value: the store keeps
// no audit, the caller persists them in its own (DAT-18, DAT-25).
type HoldReceipt struct {
	Ref    ObjectRef
	On     bool
	Reason string
	At     Instant
}

// RetainedUntil is the retention the purged version had, zero when it had none.
type PurgeReceipt struct {
	Ref           ObjectRef
	PurgedAt      Instant
	RetainedUntil Instant
}

type ObjectWriter interface {
	Put(ctx context.Context, req PutRequest) (ObjectVersion, error)
}

type ObjectReader interface {
	Get(ctx context.Context, ref ObjectRef) (Object, error)
	Head(ctx context.Context, ref ObjectRef) (ObjectInfo, error)
}

// RetentionManager only extends and promotes (GOVERNANCE to COMPLIANCE); it has
// no shortening, downgrade or bypass (DAT-14, DAT-16).
type RetentionManager interface {
	Retain(ctx context.Context, ref ObjectRef, to Retention) (ObjectInfo, error)
	SetLegalHold(ctx context.Context, ref ObjectRef, hold LegalHold) (HoldReceipt, error)
}

// ObjectPurger removes one free version; there is no delete without a version,
// because a delete marker removes nothing.
type ObjectPurger interface {
	Purge(ctx context.Context, ref ObjectRef) (PurgeReceipt, error)
}

// ObjectStore composes the four; a caller receives only the interface it needs,
// so its privilege is the one of that interface (DAT-11).
type ObjectStore interface {
	ObjectWriter
	ObjectReader
	RetentionManager
	ObjectPurger
}

var ErrObjectKeyInvalid = errors.New("ports: object key is empty, absolute or has a dot segment")

var ErrVersionRequired = errors.New("ports: object reference without a version")

var ErrIntegrity = errors.New("ports: object body does not match its SHA-256")

var ErrRetentionUnbounded = errors.New("ports: retention without a future date")

var ErrRetentionAboveCeiling = errors.New("ports: retention past the declared ceiling")

var ErrRetentionNotExtended = errors.New("ports: retention neither extended nor promoted")

// ErrRetentionModeChange reports a downgrade from COMPLIANCE to GOVERNANCE,
// which S3 refuses and the port never promises.
var ErrRetentionModeChange = errors.New("ports: retention mode cannot be downgraded")

var ErrRetentionModeUnknown = errors.New("ports: retention mode is neither COMPLIANCE nor GOVERNANCE")

var ErrObjectLocked = errors.New("ports: object version under retention or legal hold")

// ErrScopeMismatch reports a platform store reached by a context that carries a
// tenant: the platform chain neither reads nor invents one (IDN-20).
var ErrScopeMismatch = errors.New("ports: execution context does not match the store scope")

var ErrScopeUndeclared = errors.New("ports: object store scope not declared (IDN-17)")

var ErrEncryptionUndeclared = errors.New("ports: object store encryption not declared (DAT-08)")

// ErrRetentionCeilingUndeclared refuses a retention on a store built without a
// ceiling, and a ceiling declared without a positive span or an origin (DAT-17).
var ErrRetentionCeilingUndeclared = errors.New("ports: retention ceiling not declared (DAT-17)")

var ErrLegalHoldReasonRequired = errors.New("ports: legal hold without a declared reason (DAT-15)")

// ErrObjectNotFound is also the answer to another tenant's or scope's reference,
// and it unwraps to ErrNotFound so both read the same (IDN-13).
var ErrObjectNotFound error = objectNotFound{}

type objectNotFound struct{}

func (objectNotFound) Error() string { return "ports: object version not found" }

func (objectNotFound) Unwrap() error { return ErrNotFound }
