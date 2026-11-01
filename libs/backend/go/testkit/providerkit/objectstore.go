package providerkit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ObjectStoreSubject opens stores over one shared resource. Encryption is what
// they declare (zero takes the kit's); OtherEncryption, PhysicalKeys and Advance
// enable proofs that are otherwise skipped by name; RetentionUnit (zero is one
// day) is the span every retention the kit writes is a multiple of.
type ObjectStoreSubject struct {
	Open             func(decl ports.ObjectStoreDeclaration) (ports.ObjectStore, error)
	Advance          func(nanos int64) error
	TenantUnresolved error
	Encryption       ports.Encryption
	OtherEncryption  ports.Encryption
	PhysicalKeys     func() ([]string, error)
	RetentionUnit    ports.RetentionSpan
}

const (
	clausePutVersion        = "put.version: two writes to one key yield distinct versionIds, each read back by its own, and no reference goes without a version"
	clausePutIntegrity      = "put.integrity: a body that does not hash to the declared SHA-256 is refused and nothing is written"
	clauseIntegrityNoKey    = "put.integrity: a write refused for its hash leaves no key behind"
	clauseRetentionDated    = "retention.dated: a retention without a future date, past the ceiling or on a store without a ceiling is refused, on Put and on Retain"
	clauseRetentionNoKey    = "retention.dated: a write refused for its retention leaves no key behind"
	clauseCompliance        = "retention.compliance: a COMPLIANCE version is neither purged, shortened, downgraded nor extended past the ceiling, and is extended within it"
	clauseGovernance        = "retention.governance: a GOVERNANCE version is neither purged nor shortened, is extended and promoted within the ceiling, and the port has no bypass"
	clauseHoldIndependent   = "hold.independent: the hold needs a reason, keeps Purge refused, leaves the retention untouched and, lifted, frees the version"
	clauseHoldAfterExpiry   = "hold.independent: the hold keeps Purge refused after the retention expired"
	clausePurgeReceipt      = "purge.receipt: a purge after the retention expired returns the receipt and the version is gone"
	clauseTenantScope       = "tenant.scope: another tenant's reference resolves in no operation and leaks nothing, and an unresolved tenant or an escaping key is refused"
	clausePlatformScope     = "platform.scope: a store without a scope is refused, a tenant context reaches no platform operation, and the two scopes never resolve each other"
	clausePlatformPrefix    = "platform.scope: the platform store writes under platform/, a tenant under tenant/<id>/"
	clauseHeadState         = "head.state: Head reports the retention, the hold and the encryption of the stored version"
	clauseHeadStoredCipher  = "head.state: Head reports the encryption the version was stored with, not the reading store's"
	clauseOverwriteLocked   = "overwrite.keeps-locked: a write over the key of a locked version creates a new version and the old one stays locked"
	objectRule              = "SPEC-CWBA4BPV"
	kitSecond               = ports.RetentionSpan(1_000_000_000)
	kitDefaultRetentionUnit = 24 * 60 * 60 * kitSecond
	kitCeilingUnits         = 4
	kitObjectKey            = ports.ObjectKey("dmpf-kit/seal")
	kitClockKey             = ports.ObjectKey("dmpf-kit/clock")
	kitRefusedKey           = ports.ObjectKey("dmpf-kit/refused")
	kitEscapingKey          = ports.ObjectKey("../dmpf-kit-b/escape")
	kitContentType          = "application/x-dmpf-kit"
	kitHoldReason           = "dmpf-kit-litigation"
)

var kitEncryption = ports.Encryption{Algorithm: "aws:kms", KeyRef: "dmpf-kit-key"}

// ObjectStore certifies ports.ObjectStore with the clauses of SPEC-CWBA4BPV, the
// same against the fake, the emulator and real S3. Each clause takes a fresh
// subject and lifts every hold it puts, so no version outlives its retention.
func ObjectStore(newSubject func() ObjectStoreSubject) Verdict {
	var v Verdict
	c, err := newObjectContexts()
	if err != nil {
		v.fail("setup", objectRule, "contexts: %v", err)
		return v
	}

	objectPutVersion(newSubject, c, &v)
	objectPutIntegrity(newSubject, c, &v)
	objectRetentionDated(newSubject, c, &v)
	objectRetentionCompliance(newSubject, c, &v)
	objectRetentionGovernance(newSubject, c, &v)
	objectHoldIndependent(newSubject, c, &v)
	objectPurgeReceipt(newSubject, c, &v)
	objectTenantScope(newSubject, c, &v)
	objectPlatformScope(newSubject, c, &v)
	objectHeadState(newSubject, c, &v)
	objectOverwriteKeepsLocked(newSubject, c, &v)
	return v
}

type objectContexts struct {
	tenantA, tenantB, tenantless, slashed, platform, namedPlatform context.Context
}

func newObjectContexts() (objectContexts, error) {
	var c objectContexts
	var err error
	if c.tenantA, err = scopedTo(kitTenantA); err != nil {
		return c, err
	}
	if c.tenantB, err = scopedTo(kitTenantB); err != nil {
		return c, err
	}
	if c.tenantless, err = tenantless(); err != nil {
		return c, err
	}
	if c.slashed, err = scopedTo("dmpf-kit/slashed"); err != nil {
		return c, err
	}
	if c.namedPlatform, err = scopedTo("platform"); err != nil {
		return c, err
	}
	c.platform, err = platformContext()
	return c, err
}

// platformContext is the platform chain of IDN-16 and IDN-18: no subject and no
// tenant, and nothing invented for either (IDN-20).
func platformContext() (context.Context, error) {
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     "dmpf-kit-request",
		CorrelationID: "dmpf-kit-correlation",
		TraceContext:  "dmpf-kit-trace",
		Deadline:      kitDeadline,
		Locale:        "en",
	})
	if err != nil {
		return nil, err
	}
	return ports.WithExecutionContext(context.Background(), execution), nil
}

func kitObjectBody(marker string) ([]byte, string) {
	body := []byte("dmpf-kit-object:" + marker)
	sum := sha256.Sum256(body)
	return body, hex.EncodeToString(sum[:])
}

func kitPut(ctx context.Context, store ports.ObjectStore, key ports.ObjectKey, marker string, retention *ports.Retention) (ports.ObjectVersion, error) {
	body, sum := kitObjectBody(marker)
	return store.Put(ctx, ports.PutRequest{Key: key, Body: body, ContentSHA256: sum, ContentType: kitContentType, Retention: retention})
}

func sameRetention(got *ports.Retention, want ports.Retention) bool {
	return got != nil && *got == want
}

// objectCase is one clause over one subject: it opens the stores, learns the
// clock and reports, failing the clause and never the run.
type objectCase struct {
	s          ObjectStoreSubject
	v          *Verdict
	clause     string
	rule       string
	unit       ports.RetentionSpan
	encryption ports.Encryption
}

func newObjectCase(newSubject func() ObjectStoreSubject, v *Verdict, clause, rule string) (objectCase, bool) {
	s := newSubject()
	unit := s.RetentionUnit
	if unit == 0 {
		unit = kitDefaultRetentionUnit
	}
	if unit < kitSecond || unit%kitSecond != 0 {
		v.fail(clause, objectRule, "RetentionUnit = %d ns, want zero or a whole number of seconds", s.RetentionUnit)
		return objectCase{}, false
	}
	encryption := s.Encryption
	if encryption == (ports.Encryption{}) {
		encryption = kitEncryption
	}
	return objectCase{s: s, v: v, clause: clause, rule: rule, unit: unit, encryption: encryption}, true
}

func (oc objectCase) declaration(scope ports.Scope) ports.ObjectStoreDeclaration {
	return ports.ObjectStoreDeclaration{
		Scope:      scope,
		Encryption: oc.encryption,
		Ceiling:    &ports.RetentionCeiling{Span: kitCeilingUnits * oc.unit, Origin: "dmpf-kit-ceiling"},
	}
}

func (oc objectCase) after(base ports.Instant, units int64) ports.Instant {
	sum, _ := (ports.RetentionSpan(units) * oc.unit).Add(base)
	return sum
}

func (oc objectCase) open(scope ports.Scope) (ports.ObjectStore, bool) {
	store, err := oc.s.Open(oc.declaration(scope))
	if err != nil || store == nil {
		oc.v.fail(oc.clause, oc.rule, "Open(%s) = %v, want a store", scope, err)
		return nil, false
	}
	return store, true
}

// now is the store's clock, read from a probe write and floored to the second
// that every retention date is kept to.
func (oc objectCase) now(ctx context.Context, store ports.ObjectStore) (ports.Instant, bool) {
	probe, err := kitPut(ctx, store, kitClockKey, "clock", nil)
	if err != nil {
		oc.v.fail(oc.clause, oc.rule, "Put(clock probe) = %v, want nil", err)
		return 0, false
	}
	return probe.StoredAt - probe.StoredAt%ports.Instant(kitSecond), true
}

func (oc objectCase) put(ctx context.Context, store ports.ObjectStore, marker string, retention *ports.Retention) (ports.ObjectVersion, bool) {
	version, err := kitPut(ctx, store, kitObjectKey, marker, retention)
	if err != nil {
		oc.v.fail(oc.clause, oc.rule, "Put(%s) = %v, want nil", marker, err)
		return ports.ObjectVersion{}, false
	}
	return version, true
}

func (oc objectCase) refused(rule, op string, err, want error) bool {
	if !errors.Is(err, want) {
		oc.v.fail(oc.clause, rule, "%s = %v, want %v", op, err, want)
		return false
	}
	return true
}

func (oc objectCase) refusedBare(rule, op string, err, want error, bare bool) {
	if oc.refused(rule, op, err, want) && !bare {
		oc.v.fail(oc.clause, rule, "%s returned a value with the refusal, want the zero value: nothing of the version may leak", op)
	}
}

func (oc objectCase) accepted(rule, op string, err error) bool {
	if err != nil {
		oc.v.fail(oc.clause, rule, "%s = %v, want nil", op, err)
		return false
	}
	return true
}

// refusesEvery calls, as who, the five operations that take a reference, wanting
// want from each and the zero value with it, so no byte nor state of it leaks.
func (oc objectCase) refusesEvery(rule, who string, ctx context.Context, store ports.ObjectStore, ref ports.ObjectRef, until ports.Instant, want error) {
	object, err := store.Get(ctx, ref)
	oc.refusedBare(rule, "Get("+who+")", err, want, len(object.Body) == 0 && object.ObjectInfo == ports.ObjectInfo{})
	info, err := store.Head(ctx, ref)
	oc.refusedBare(rule, "Head("+who+")", err, want, info == ports.ObjectInfo{})
	retained, err := store.Retain(ctx, ref, ports.Retention{Mode: ports.RetentionCompliance, Until: until})
	oc.refusedBare(rule, "Retain("+who+")", err, want, retained == ports.ObjectInfo{})
	hold, err := store.SetLegalHold(ctx, ref, ports.LegalHold{On: true, Reason: kitHoldReason})
	oc.refusedBare(rule, "SetLegalHold("+who+")", err, want, hold == ports.HoldReceipt{})
	purged, err := store.Purge(ctx, ref)
	oc.refusedBare(rule, "Purge("+who+")", err, want, purged == ports.PurgeReceipt{})
}

// leftNoKey proves, when the subject lists its physical keys, that no refused
// write left key behind; without the listing the proof is skipped by name.
func (oc objectCase) leftNoKey(proof string, key ports.ObjectKey) {
	if oc.s.PhysicalKeys == nil {
		oc.v.skip(proof)
		return
	}
	keys, err := oc.s.PhysicalKeys()
	if err != nil {
		oc.v.fail(proof, oc.rule, "PhysicalKeys() = %v, want the keys of the resource", err)
		return
	}
	for _, physical := range keys {
		if strings.HasSuffix(physical, "/"+string(key)) {
			oc.v.fail(proof, oc.rule, "physical key %q exists after the refused writes, want none", physical)
			return
		}
	}
}

func (oc objectCase) liftHold(ctx context.Context, store ports.ObjectStore, ref ports.ObjectRef) {
	if _, err := store.SetLegalHold(ctx, ref, ports.LegalHold{}); err != nil {
		oc.v.fail(oc.clause, "DAT-15", "SetLegalHold(off, cleanup) = %v, want nil: a hold left on never expires", err)
	}
}

func (oc objectCase) advance(span ports.RetentionSpan) bool {
	if err := oc.s.Advance(int64(span)); err != nil {
		oc.v.fail(oc.clause, oc.rule, "Advance(%d) = %v, want nil", span, err)
		return false
	}
	return true
}

func objectPutVersion(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clausePutVersion, objectRule)
	if !ok {
		return
	}
	store, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	first, ok := oc.put(c.tenantA, store, "first", nil)
	if !ok {
		return
	}
	second, ok := oc.put(c.tenantA, store, "second", nil)
	if !ok {
		return
	}
	if first.Version == "" || first.Version == second.Version {
		v.fail(oc.clause, objectRule, "versions %q and %q, want two distinct, non-empty versionIds", first.Version, second.Version)
		return
	}
	for _, written := range []struct {
		version ports.ObjectVersion
		marker  string
	}{{first, "first"}, {second, "second"}} {
		body, sum := kitObjectBody(written.marker)
		if written.version.Key != kitObjectKey || written.version.ContentSHA256 != sum || written.version.Size != int64(len(body)) {
			v.fail(oc.clause, objectRule, "Put(%s) = %+v, want key %q, hash %s and size %d", written.marker, written.version, kitObjectKey, sum, len(body))
			return
		}
		got, err := store.Get(c.tenantA, written.version.Ref())
		if err != nil || !bytes.Equal(got.Body, body) || got.Version != written.version.Version {
			v.fail(oc.clause, objectRule, "Get(%s) = %q at %q, %v; want %q at %q", written.marker, got.Body, got.Version, err, body, written.version.Version)
			return
		}
	}
	oc.refusesEvery(objectRule, "no version", c.tenantA, store, ports.ObjectRef{Key: kitObjectKey}, oc.after(first.StoredAt, 1), ports.ErrVersionRequired)
}

func objectPutIntegrity(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clausePutIntegrity, objectRule)
	if !ok {
		return
	}
	store, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	body, _ := kitObjectBody("tampered")
	_, other := kitObjectBody("other")
	for _, declared := range []struct{ name, sum string }{{"the hash of another body", other}, {"no hash", ""}} {
		got, err := store.Put(c.tenantA, ports.PutRequest{Key: kitRefusedKey, Body: body, ContentSHA256: declared.sum, ContentType: kitContentType})
		if !oc.refused(objectRule, "Put("+declared.name+")", err, ports.ErrIntegrity) {
			return
		}
		if got != (ports.ObjectVersion{}) {
			v.fail(oc.clause, objectRule, "Put(%s) returned %+v with the refusal, want no version", declared.name, got)
			return
		}
	}
	oc.leftNoKey(clauseIntegrityNoKey, kitRefusedKey)
	if written, ok := oc.put(c.tenantA, store, "tampered", nil); ok && written.Version == "" {
		v.fail(oc.clause, objectRule, "Put(the right hash) returned no versionId")
	}
}

func objectRetentionDated(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clauseRetentionDated, "DAT-14")
	if !ok {
		return
	}
	store, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	base, ok := oc.now(c.tenantA, store)
	if !ok {
		return
	}
	probe, ok := oc.put(c.tenantA, store, "free", nil)
	if !ok {
		return
	}
	refusals := []struct {
		name      string
		retention ports.Retention
		want      error
		rule      string
	}{
		{"no date", ports.Retention{Mode: ports.RetentionCompliance}, ports.ErrRetentionUnbounded, "DAT-17"},
		{"a past date", ports.Retention{Mode: ports.RetentionCompliance, Until: base - ports.Instant(kitSecond)}, ports.ErrRetentionUnbounded, "DAT-17"},
		{"past the ceiling", ports.Retention{Mode: ports.RetentionGovernance, Until: oc.after(base, kitCeilingUnits+1)}, ports.ErrRetentionAboveCeiling, "DAT-14"},
		{"an unknown mode", ports.Retention{Mode: "FOREVER", Until: oc.after(base, 1)}, ports.ErrRetentionModeUnknown, objectRule},
	}
	for _, r := range refusals {
		got, err := kitPut(c.tenantA, store, kitRefusedKey, "refused", &r.retention)
		if oc.refused(r.rule, "Put("+r.name+")", err, r.want) && got != (ports.ObjectVersion{}) {
			v.fail(oc.clause, r.rule, "Put(%s) returned %+v with the refusal, want no version", r.name, got)
		}
		_, err = store.Retain(c.tenantA, probe.Ref(), r.retention)
		oc.refused(r.rule, "Retain("+r.name+")", err, r.want)
	}
	oc.leftNoKey(clauseRetentionNoKey, kitRefusedKey)
	if info, err := store.Head(c.tenantA, probe.Ref()); err != nil || info.Retention != nil {
		v.fail(oc.clause, "DAT-14", "Head(after refused Retains) = %+v, %v; want the version still without retention", info.Retention, err)
	}
	onCeiling := ports.Retention{Mode: ports.RetentionGovernance, Until: oc.after(base, kitCeilingUnits)}
	if _, err := kitPut(c.tenantA, store, kitObjectKey, "on the ceiling", &onCeiling); err != nil {
		v.fail(oc.clause, "DAT-14", "Put(retention exactly on the ceiling) = %v, want nil: the ceiling is inclusive", err)
	}

	unbounded, err := oc.s.Open(ports.ObjectStoreDeclaration{Scope: ports.ScopeTenant, Encryption: oc.encryption})
	if err != nil || unbounded == nil {
		v.fail(oc.clause, "DAT-17", "Open(no ceiling) = %v, want a store that refuses retention", err)
		return
	}
	free, ok := oc.put(c.tenantA, unbounded, "no ceiling", nil)
	if !ok {
		return
	}
	dated := ports.Retention{Mode: ports.RetentionGovernance, Until: oc.after(base, 1)}
	_, err = kitPut(c.tenantA, unbounded, kitObjectKey, "no ceiling", &dated)
	oc.refused("DAT-17", "Put(retention, store without ceiling)", err, ports.ErrRetentionCeilingUndeclared)
	_, err = unbounded.Retain(c.tenantA, free.Ref(), dated)
	oc.refused("DAT-17", "Retain(store without ceiling)", err, ports.ErrRetentionCeilingUndeclared)
}

func objectRetentionCompliance(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clauseCompliance, "DAT-14")
	if !ok {
		return
	}
	store, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	base, ok := oc.now(c.tenantA, store)
	if !ok {
		return
	}
	held := ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, 2)}
	locked, ok := oc.put(c.tenantA, store, "compliance", &held)
	if !ok {
		return
	}
	ref := locked.Ref()

	_, err := store.Purge(c.tenantA, ref)
	oc.refused("DAT-14", "Purge(under COMPLIANCE)", err, ports.ErrObjectLocked)
	for _, r := range []struct {
		name string
		to   ports.Retention
		want error
	}{
		{"an earlier date", ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, 1)}, ports.ErrRetentionNotExtended},
		{"the same date", held, ports.ErrRetentionNotExtended},
		{"a downgrade to GOVERNANCE", ports.Retention{Mode: ports.RetentionGovernance, Until: oc.after(base, 3)}, ports.ErrRetentionModeChange},
		{"past the ceiling", ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, kitCeilingUnits+1)}, ports.ErrRetentionAboveCeiling},
	} {
		_, err := store.Retain(c.tenantA, ref, r.to)
		oc.refused("DAT-14", "Retain("+r.name+")", err, r.want)
	}
	if info, err := store.Head(c.tenantA, ref); err != nil || !sameRetention(info.Retention, held) {
		v.fail(oc.clause, "DAT-14", "Head(after refused Retains) = %+v, %v; want %+v untouched", info.Retention, err, held)
		return
	}

	extended := ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, 3)}
	info, err := store.Retain(c.tenantA, ref, extended)
	if !oc.accepted("DAT-14", "Retain(a later date within the ceiling)", err) {
		return
	}
	if !sameRetention(info.Retention, extended) {
		v.fail(oc.clause, "DAT-14", "Retain returned %+v, want %+v", info.Retention, extended)
	}
	if head, err := store.Head(c.tenantA, ref); err != nil || !sameRetention(head.Retention, extended) {
		v.fail(oc.clause, "DAT-14", "Head(after the extension) = %+v, %v; want %+v", head.Retention, err, extended)
	}
}

func objectRetentionGovernance(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clauseGovernance, "DAT-14")
	if !ok {
		return
	}
	store, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	base, ok := oc.now(c.tenantA, store)
	if !ok {
		return
	}
	held := ports.Retention{Mode: ports.RetentionGovernance, Until: oc.after(base, 2)}
	governed, ok := oc.put(c.tenantA, store, "governance", &held)
	if !ok {
		return
	}
	ref := governed.Ref()

	_, err := store.Purge(c.tenantA, ref)
	oc.refused("DAT-11", "Purge(under GOVERNANCE, no bypass)", err, ports.ErrObjectLocked)
	for _, r := range []struct {
		name string
		to   ports.Retention
		want error
	}{
		{"an earlier date", ports.Retention{Mode: ports.RetentionGovernance, Until: oc.after(base, 1)}, ports.ErrRetentionNotExtended},
		{"the same date", held, ports.ErrRetentionNotExtended},
		{"a promotion to an earlier date", ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, 1)}, ports.ErrRetentionNotExtended},
		{"past the ceiling", ports.Retention{Mode: ports.RetentionGovernance, Until: oc.after(base, kitCeilingUnits+1)}, ports.ErrRetentionAboveCeiling},
		{"a promotion past the ceiling", ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, kitCeilingUnits+1)}, ports.ErrRetentionAboveCeiling},
	} {
		_, err := store.Retain(c.tenantA, ref, r.to)
		oc.refused("DAT-14", "Retain("+r.name+")", err, r.want)
	}

	extended := ports.Retention{Mode: ports.RetentionGovernance, Until: oc.after(base, 3)}
	if info, err := store.Retain(c.tenantA, ref, extended); !oc.accepted("DAT-14", "Retain(a later GOVERNANCE date within the ceiling)", err) {
		return
	} else if !sameRetention(info.Retention, extended) {
		v.fail(oc.clause, "DAT-14", "Retain(extension) returned %+v, want %+v", info.Retention, extended)
	}
	promoted := ports.Retention{Mode: ports.RetentionCompliance, Until: extended.Until}
	info, err := store.Retain(c.tenantA, ref, promoted)
	if !oc.accepted("DAT-14", "Retain(promotion to COMPLIANCE on the same date)", err) {
		return
	}
	if !sameRetention(info.Retention, promoted) {
		v.fail(oc.clause, "DAT-14", "Retain(promotion) returned %+v, want %+v", info.Retention, promoted)
	}
	if head, err := store.Head(c.tenantA, ref); err != nil || !sameRetention(head.Retention, promoted) {
		v.fail(oc.clause, "DAT-14", "Head(after the promotion) = %+v, %v; want %+v", head.Retention, err, promoted)
		return
	}
	_, err = store.Retain(c.tenantA, ref, ports.Retention{Mode: ports.RetentionGovernance, Until: oc.after(base, kitCeilingUnits)})
	oc.refused("DAT-14", "Retain(back to GOVERNANCE after the promotion)", err, ports.ErrRetentionModeChange)
	_, err = store.Retain(c.tenantA, ref, ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, kitCeilingUnits+1)})
	oc.refused("DAT-14", "Retain(promoted, past the ceiling)", err, ports.ErrRetentionAboveCeiling)
}

func objectHoldIndependent(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clauseHoldIndependent, "DAT-15")
	if !ok {
		return
	}
	store, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	base, ok := oc.now(c.tenantA, store)
	if !ok {
		return
	}
	free, ok := oc.put(c.tenantA, store, "held", nil)
	if !ok {
		return
	}
	ref := free.Ref()

	_, err := store.SetLegalHold(c.tenantA, ref, ports.LegalHold{On: true})
	oc.refused("DAT-15", "SetLegalHold(on, no reason)", err, ports.ErrLegalHoldReasonRequired)
	if info, err := store.Head(c.tenantA, ref); err != nil || info.LegalHold {
		v.fail(oc.clause, "DAT-15", "Head(after the refused hold) = hold %v, %v; want no hold", info.LegalHold, err)
		return
	}
	on, err := store.SetLegalHold(c.tenantA, ref, ports.LegalHold{On: true, Reason: kitHoldReason})
	if !oc.accepted("DAT-15", "SetLegalHold(on)", err) {
		return
	}
	if on.Ref != ref || !on.On || on.Reason != kitHoldReason || on.At < base {
		v.fail(oc.clause, "DAT-18", "HoldReceipt = %+v, want ref %+v, on, reason %q and an instant from %d", on, ref, kitHoldReason, base)
	}
	if info, err := store.Head(c.tenantA, ref); err != nil || !info.LegalHold || info.Retention != nil {
		v.fail(oc.clause, "DAT-15", "Head(held) = hold %v, retention %+v, %v; want the hold and no retention", info.LegalHold, info.Retention, err)
	}
	_, err = store.Purge(c.tenantA, ref)
	oc.refused("DAT-18", "Purge(under hold, no retention)", err, ports.ErrObjectLocked)
	off, err := store.SetLegalHold(c.tenantA, ref, ports.LegalHold{})
	if !oc.accepted("DAT-15", "SetLegalHold(off)", err) {
		return
	}
	if off.Ref != ref || off.On {
		v.fail(oc.clause, "DAT-18", "HoldReceipt(off) = %+v, want ref %+v and off", off, ref)
	}
	purged, err := store.Purge(c.tenantA, ref)
	if oc.accepted("DAT-18", "Purge(no hold, no retention)", err) && (purged.Ref != ref || purged.RetainedUntil != 0) {
		v.fail(oc.clause, "DAT-18", "PurgeReceipt = %+v, want ref %+v and no retention", purged, ref)
	}

	retained := ports.Retention{Mode: ports.RetentionGovernance, Until: oc.after(base, 1)}
	governed, ok := oc.put(c.tenantA, store, "held under retention", &retained)
	if !ok {
		return
	}
	if _, err := store.SetLegalHold(c.tenantA, governed.Ref(), ports.LegalHold{On: true, Reason: kitHoldReason}); !oc.accepted("DAT-15", "SetLegalHold(on, under retention)", err) {
		return
	}
	if info, err := store.Head(c.tenantA, governed.Ref()); err != nil || !info.LegalHold || !sameRetention(info.Retention, retained) {
		v.fail(oc.clause, "DAT-15", "Head(held under retention) = hold %v, retention %+v, %v; want the hold and %+v untouched", info.LegalHold, info.Retention, err, retained)
	}

	if oc.s.Advance == nil {
		oc.liftHold(c.tenantA, store, governed.Ref())
		v.skip(clauseHoldAfterExpiry)
		return
	}
	expiry := oc
	expiry.clause, expiry.rule = clauseHoldAfterExpiry, "DAT-18"
	if !expiry.advance(oc.unit + 1) {
		oc.liftHold(c.tenantA, store, governed.Ref())
		return
	}
	_, err = store.Purge(c.tenantA, governed.Ref())
	expiry.refused("DAT-18", "Purge(held, retention expired)", err, ports.ErrObjectLocked)
	if _, err := store.SetLegalHold(c.tenantA, governed.Ref(), ports.LegalHold{}); !expiry.accepted("DAT-15", "SetLegalHold(off, retention expired)", err) {
		return
	}
	purged, err = store.Purge(c.tenantA, governed.Ref())
	if expiry.accepted("DAT-18", "Purge(hold lifted, retention expired)", err) && purged.RetainedUntil != retained.Until {
		v.fail(clauseHoldAfterExpiry, "DAT-18", "PurgeReceipt.RetainedUntil = %d, want %d", purged.RetainedUntil, retained.Until)
	}
}

func objectPurgeReceipt(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clausePurgeReceipt, "DAT-18")
	if !ok {
		return
	}
	if oc.s.Advance == nil {
		v.skip(clausePurgeReceipt)
		return
	}
	store, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	base, ok := oc.now(c.tenantA, store)
	if !ok {
		return
	}
	retained := ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, 1)}
	locked, ok := oc.put(c.tenantA, store, "expiring", &retained)
	if !ok || !oc.advance(oc.unit+1) {
		return
	}
	ref := locked.Ref()
	receipt, err := store.Purge(c.tenantA, ref)
	if !oc.accepted("DAT-18", "Purge(retention expired)", err) {
		return
	}
	if receipt.Ref != ref || receipt.RetainedUntil != retained.Until || receipt.PurgedAt < retained.Until {
		v.fail(oc.clause, "DAT-18", "PurgeReceipt = %+v, want ref %+v, retained until %d and purged from then on", receipt, ref, retained.Until)
	}
	got, err := store.Get(c.tenantA, ref)
	if oc.refused("DAT-18", "Get(purged)", err, ports.ErrObjectNotFound) && len(got.Body) != 0 {
		v.fail(oc.clause, "DAT-18", "Get(purged) = %q, want no body", got.Body)
	}
	_, err = store.Purge(c.tenantA, ref)
	oc.refused("DAT-18", "Purge(purged)", err, ports.ErrObjectNotFound)
}

func objectTenantScope(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clauseTenantScope, "IDN-13")
	if !ok {
		return
	}
	if oc.s.TenantUnresolved == nil {
		v.fail(oc.clause, "IDN-15", "the subject names no TenantUnresolved sentinel")
		return
	}
	store, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	base, ok := oc.now(c.tenantA, store)
	if !ok {
		return
	}
	held := ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, 2)}
	owned, ok := oc.put(c.tenantA, store, "tenant a", &held)
	if !ok {
		return
	}
	ref := owned.Ref()
	extension := oc.after(base, 3)

	oc.refusesEvery("IDN-13", "tenant b, a's reference", c.tenantB, store, ref, extension, ports.ErrObjectNotFound)
	body, _ := kitObjectBody("tenant a")
	if read, err := store.Get(c.tenantA, ref); err != nil || !bytes.Equal(read.Body, body) || !sameRetention(read.Retention, held) || read.LegalHold {
		v.fail(oc.clause, "IDN-14", "Get(tenant a) = %q, retention %+v, hold %v, %v; want a's version intact", read.Body, read.Retention, read.LegalHold, err)
	}
	_, err := store.Purge(c.tenantA, ref)
	oc.refused("IDN-14", "Purge(tenant a, still locked)", err, ports.ErrObjectLocked)

	for _, caller := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"no tenant", c.tenantless, oc.s.TenantUnresolved},
		{"a tenant with a slash", c.slashed, oc.s.TenantUnresolved},
		{"no execution context", context.Background(), ports.ErrContextAbsent},
	} {
		_, err := kitPut(caller.ctx, store, kitObjectKey, "unscoped", nil)
		oc.refused("IDN-15", "Put("+caller.name+")", err, caller.want)
		oc.refusesEvery("IDN-15", caller.name, caller.ctx, store, ref, extension, caller.want)
	}

	for _, key := range []ports.ObjectKey{kitEscapingKey, ""} {
		_, err := kitPut(c.tenantA, store, key, "escaping", nil)
		oc.refused("IDN-14", "Put(key "+string(key)+")", err, ports.ErrObjectKeyInvalid)
	}
	oc.refusesEvery("IDN-14", "an escaping key", c.tenantA, store, ports.ObjectRef{Key: kitEscapingKey, Version: ref.Version}, extension, ports.ErrObjectKeyInvalid)
}

func objectPlatformScope(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clausePlatformScope, "IDN-19")
	if !ok {
		return
	}
	for _, scope := range []ports.Scope{"", "global"} {
		if store, err := oc.s.Open(oc.declaration(scope)); !errors.Is(err, ports.ErrScopeUndeclared) || store != nil {
			v.fail(oc.clause, "IDN-17", "Open(scope %q) = %v, %v; want no store and ErrScopeUndeclared", scope, store, err)
		}
	}
	platform, ok := oc.open(ports.ScopePlatform)
	if !ok {
		return
	}
	tenant, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	base, ok := oc.now(c.platform, platform)
	if !ok {
		return
	}
	extension := oc.after(base, 1)

	seal, err := kitPut(c.platform, platform, kitObjectKey, "platform seal", nil)
	if !oc.accepted("IDN-16", "Put(platform chain, no subject nor tenant)", err) {
		return
	}
	sealBody, _ := kitObjectBody("platform seal")
	if got, err := platform.Get(c.platform, seal.Ref()); err != nil || !bytes.Equal(got.Body, sealBody) {
		v.fail(oc.clause, "IDN-18", "Get(platform chain) = %q, %v; want the seal", got.Body, err)
	}
	_, err = kitPut(c.tenantA, platform, kitObjectKey, "tenant in platform", nil)
	oc.refused("IDN-20", "Put(platform store, context with a tenant)", err, ports.ErrScopeMismatch)
	oc.refusesEvery("IDN-20", "platform store, context with a tenant", c.tenantA, platform, seal.Ref(), extension, ports.ErrScopeMismatch)
	oc.refusesEvery("IDN-19", "tenant store, tenant a, the platform reference", c.tenantA, tenant, seal.Ref(), extension, ports.ErrObjectNotFound)
	oc.refusesEvery("IDN-19", `tenant store, the tenant named "platform", the platform reference`, c.namedPlatform, tenant, seal.Ref(), extension, ports.ErrObjectNotFound)

	shadow, err := kitPut(c.namedPlatform, tenant, kitObjectKey, "shadow", nil)
	if !oc.accepted("IDN-19", `Put(tenant store, the tenant named "platform")`, err) {
		return
	}
	oc.refusesEvery("IDN-19", "platform store, a tenant reference", c.platform, platform, shadow.Ref(), extension, ports.ErrObjectNotFound)
	if got, err := platform.Get(c.platform, seal.Ref()); err != nil || !bytes.Equal(got.Body, sealBody) || got.LegalHold || got.Retention != nil {
		v.fail(oc.clause, "IDN-19", "Get(platform seal after every refused reach) = %q, hold %v, retention %+v, %v; want the seal untouched", got.Body, got.LegalHold, got.Retention, err)
	}
	shadowBody, _ := kitObjectBody("shadow")
	if got, err := tenant.Get(c.namedPlatform, shadow.Ref()); err != nil || !bytes.Equal(got.Body, shadowBody) || got.LegalHold || got.Retention != nil {
		v.fail(oc.clause, "IDN-19", `Get(the tenant named "platform") = %q, hold %v, retention %+v, %v; want its version untouched`, got.Body, got.LegalHold, got.Retention, err)
	}

	if oc.s.PhysicalKeys == nil {
		v.skip(clausePlatformPrefix)
		return
	}
	keys, err := oc.s.PhysicalKeys()
	if err != nil {
		v.fail(clausePlatformPrefix, "IDN-19", "PhysicalKeys() = %v, want the keys of the resource", err)
		return
	}
	for _, want := range []string{"platform/" + string(kitObjectKey), "tenant/platform/" + string(kitObjectKey)} {
		if !slices.Contains(keys, want) {
			v.fail(clausePlatformPrefix, "IDN-19", "physical keys %v, want %q among them", keys, want)
		}
	}
}

func objectHeadState(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clauseHeadState, "DAT-08")
	if !ok {
		return
	}
	plain := oc.declaration(ports.ScopeTenant)
	plain.Encryption = ports.Encryption{}
	if store, err := oc.s.Open(plain); !errors.Is(err, ports.ErrEncryptionUndeclared) || store != nil {
		v.fail(oc.clause, "DAT-08", "Open(no encryption) = %v, %v; want no store and ErrEncryptionUndeclared", store, err)
	}
	store, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	base, ok := oc.now(c.tenantA, store)
	if !ok {
		return
	}
	held := ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, 2)}
	written, ok := oc.put(c.tenantA, store, "inspected", &held)
	if !ok {
		return
	}
	if _, err := store.SetLegalHold(c.tenantA, written.Ref(), ports.LegalHold{On: true, Reason: kitHoldReason}); !oc.accepted("DAT-15", "SetLegalHold(on)", err) {
		return
	}
	defer oc.liftHold(c.tenantA, store, written.Ref())
	info, err := store.Head(c.tenantA, written.Ref())
	if !oc.accepted("DAT-08", "Head", err) {
		return
	}
	if info.ObjectVersion != written || info.ContentType != kitContentType || !sameRetention(info.Retention, held) || !info.LegalHold || info.Encryption != oc.encryption {
		v.fail(oc.clause, "DAT-08", "Head = %+v (retention %+v), want version %+v, type %q, retention %+v, the hold and encryption %+v",
			info, info.Retention, written, kitContentType, held, oc.encryption)
	}
	if got, err := store.Get(c.tenantA, written.Ref()); err != nil || got.ObjectVersion != info.ObjectVersion || got.Encryption != info.Encryption || got.LegalHold != info.LegalHold || !sameRetention(got.Retention, held) {
		v.fail(oc.clause, "DAT-08", "Get = %+v, %v; want the same state Head reports", got.ObjectInfo, err)
	}

	other := oc.s.OtherEncryption
	if other == (ports.Encryption{}) {
		v.skip(clauseHeadStoredCipher)
		return
	}
	if other == oc.encryption {
		v.fail(clauseHeadStoredCipher, "DAT-08", "OtherEncryption equals Encryption (%+v), want two distinct encryptions", other)
		return
	}
	declared := oc.declaration(ports.ScopeTenant)
	declared.Encryption = other
	reader, err := oc.s.Open(declared)
	if err != nil || reader == nil {
		v.fail(clauseHeadStoredCipher, "DAT-08", "Open(other encryption) = %v, want a store", err)
		return
	}
	if got, err := reader.Head(c.tenantA, written.Ref()); err != nil || got.Encryption != oc.encryption {
		v.fail(clauseHeadStoredCipher, "DAT-08", "Head(through a store declaring %+v) = %+v, %v; want %+v, the version's", other, got.Encryption, err, oc.encryption)
	}
	rewritten, err := kitPut(c.tenantA, reader, kitObjectKey, "other encryption", nil)
	if err != nil {
		v.fail(clauseHeadStoredCipher, "DAT-08", "Put(other encryption) = %v, want nil", err)
		return
	}
	if got, err := store.Head(c.tenantA, rewritten.Ref()); err != nil || got.Encryption != other {
		v.fail(clauseHeadStoredCipher, "DAT-08", "Head(the other version, through the first store) = %+v, %v; want %+v", got.Encryption, err, other)
	}
}

func objectOverwriteKeepsLocked(newSubject func() ObjectStoreSubject, c objectContexts, v *Verdict) {
	oc, ok := newObjectCase(newSubject, v, clauseOverwriteLocked, "floci#4389")
	if !ok {
		return
	}
	store, ok := oc.open(ports.ScopeTenant)
	if !ok {
		return
	}
	base, ok := oc.now(c.tenantA, store)
	if !ok {
		return
	}
	held := ports.Retention{Mode: ports.RetentionCompliance, Until: oc.after(base, 2)}
	old, ok := oc.put(c.tenantA, store, "locked", &held)
	if !ok {
		return
	}
	newer, ok := oc.put(c.tenantA, store, "overwrite", nil)
	if !ok {
		return
	}
	if newer.Version == old.Version {
		v.fail(oc.clause, oc.rule, "the overwrite reused versionId %q, want a new version", old.Version)
		return
	}
	if info, err := store.Head(c.tenantA, old.Ref()); err != nil || !sameRetention(info.Retention, held) {
		v.fail(oc.clause, oc.rule, "Head(old) = %+v, %v; want %+v untouched", info.Retention, err, held)
	}
	_, err := store.Purge(c.tenantA, old.Ref())
	oc.refused(oc.rule, "Purge(old, still locked)", err, ports.ErrObjectLocked)
	oldBody, _ := kitObjectBody("locked")
	if got, err := store.Get(c.tenantA, old.Ref()); err != nil || !bytes.Equal(got.Body, oldBody) {
		v.fail(oc.clause, oc.rule, "Get(old) = %q, %v; want %q", got.Body, err, oldBody)
	}
	if _, err := store.Purge(c.tenantA, newer.Ref()); err != nil {
		v.fail(oc.clause, oc.rule, "Purge(the free overwrite) = %v, want nil", err)
	}
}
