package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var ErrIncompleteObjectStore = errors.New("memory: object store requires a bucket with a clock")

const retentionResolution = ports.Instant(1_000_000_000)

// ObjectBucket is the storage several ObjectStores share, as stores of both
// scopes share one S3 bucket. Like the S3 server, the bucket owns the clock
// that stamps versions and decides when a retention expires.
type ObjectBucket struct {
	clock ports.Clock

	mu       sync.Mutex
	versions map[objectAddress]*storedVersion
	issued   int
}

type objectAddress struct {
	physical string
	version  ports.VersionID
}

type storedVersion struct {
	info ports.ObjectInfo
	body []byte
}

func NewObjectBucket(clock ports.Clock) *ObjectBucket {
	return &ObjectBucket{clock: clock, versions: map[objectAddress]*storedVersion{}}
}

// PhysicalKeys lists, sorted and once each, the keys as the bucket holds them,
// prefix included: the only view of the prefixes the scopes write under.
func (b *ObjectBucket) PhysicalKeys() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	keys := make([]string, 0, len(b.versions))
	for address := range b.versions {
		keys = append(keys, address.physical)
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

// WHY: as in Store.Within, a caller whose context dies while queued on the lock
// is refused again when its turn comes, so it never touches the bucket.
func (b *ObjectBucket) acquire(ctx context.Context) error {
	b.mu.Lock()
	if err := ctx.Err(); err != nil {
		b.mu.Unlock()
		return err
	}
	return nil
}

// ObjectStore is the fake of ports.ObjectStore: the same refusals as a real
// provider, with the bucket's clock deciding when a retention expires.
type ObjectStore struct {
	bucket *ObjectBucket
	decl   ports.ObjectStoreDeclaration
}

var _ ports.ObjectStore = (*ObjectStore)(nil)

func NewObjectStore(bucket *ObjectBucket, decl ports.ObjectStoreDeclaration) (*ObjectStore, error) {
	if bucket == nil || bucket.clock == nil {
		return nil, ErrIncompleteObjectStore
	}
	if err := decl.Validate(); err != nil {
		return nil, err
	}
	if decl.Ceiling != nil {
		ceiling := *decl.Ceiling
		decl.Ceiling = &ceiling
	}
	return &ObjectStore{bucket: bucket, decl: decl}, nil
}

func (s *ObjectStore) Put(ctx context.Context, req ports.PutRequest) (ports.ObjectVersion, error) {
	if err := req.Key.Validate(); err != nil {
		return ports.ObjectVersion{}, err
	}
	physical, err := s.physical(ctx, req.Key)
	if err != nil {
		return ports.ObjectVersion{}, err
	}
	sum := sha256.Sum256(req.Body)
	digest := hex.EncodeToString(sum[:])
	if req.ContentSHA256 != digest {
		return ports.ObjectVersion{}, ports.ErrIntegrity
	}
	now := s.bucket.clock.Now()
	var retention *ports.Retention
	if req.Retention != nil {
		dated := wholeSeconds(*req.Retention)
		if err := s.checkRetention(now, dated); err != nil {
			return ports.ObjectVersion{}, err
		}
		retention = &dated
	}

	if err := s.bucket.acquire(ctx); err != nil {
		return ports.ObjectVersion{}, err
	}
	defer s.bucket.mu.Unlock()
	s.bucket.issued++
	version := ports.ObjectVersion{
		Key:           req.Key,
		Version:       ports.VersionID(fmt.Sprintf("v-%06d", s.bucket.issued)),
		ContentSHA256: digest,
		Size:          int64(len(req.Body)),
		StoredAt:      now,
	}
	s.bucket.versions[objectAddress{physical: physical, version: version.Version}] = &storedVersion{
		info: ports.ObjectInfo{ObjectVersion: version, ContentType: req.ContentType, Retention: retention, Encryption: s.decl.Encryption},
		body: slices.Clone(req.Body),
	}
	return version, nil
}

func (s *ObjectStore) Get(ctx context.Context, ref ports.ObjectRef) (ports.Object, error) {
	address, err := s.locate(ctx, ref)
	if err != nil {
		return ports.Object{}, err
	}
	if err := s.bucket.acquire(ctx); err != nil {
		return ports.Object{}, err
	}
	defer s.bucket.mu.Unlock()
	stored, ok := s.bucket.versions[address]
	if !ok {
		return ports.Object{}, ports.ErrObjectNotFound
	}
	return ports.Object{ObjectInfo: stored.snapshot(), Body: slices.Clone(stored.body)}, nil
}

func (s *ObjectStore) Head(ctx context.Context, ref ports.ObjectRef) (ports.ObjectInfo, error) {
	address, err := s.locate(ctx, ref)
	if err != nil {
		return ports.ObjectInfo{}, err
	}
	if err := s.bucket.acquire(ctx); err != nil {
		return ports.ObjectInfo{}, err
	}
	defer s.bucket.mu.Unlock()
	stored, ok := s.bucket.versions[address]
	if !ok {
		return ports.ObjectInfo{}, ports.ErrObjectNotFound
	}
	return stored.snapshot(), nil
}

// Retain extends, or promotes GOVERNANCE to COMPLIANCE on a date no earlier.
// Once the current retention expired, the version is free and takes any new one.
func (s *ObjectStore) Retain(ctx context.Context, ref ports.ObjectRef, to ports.Retention) (ports.ObjectInfo, error) {
	address, err := s.locate(ctx, ref)
	if err != nil {
		return ports.ObjectInfo{}, err
	}
	now := s.bucket.clock.Now()
	to = wholeSeconds(to)
	if err := s.checkRetention(now, to); err != nil {
		return ports.ObjectInfo{}, err
	}

	if err := s.bucket.acquire(ctx); err != nil {
		return ports.ObjectInfo{}, err
	}
	defer s.bucket.mu.Unlock()
	stored, ok := s.bucket.versions[address]
	if !ok {
		return ports.ObjectInfo{}, ports.ErrObjectNotFound
	}
	if current := stored.info.Retention; current != nil && current.Until > now {
		if current.Mode == ports.RetentionCompliance && to.Mode == ports.RetentionGovernance {
			return ports.ObjectInfo{}, ports.ErrRetentionModeChange
		}
		promoted := current.Mode == ports.RetentionGovernance && to.Mode == ports.RetentionCompliance
		if to.Until < current.Until || (to.Until == current.Until && !promoted) {
			return ports.ObjectInfo{}, ports.ErrRetentionNotExtended
		}
	}
	stored.info.Retention = &to
	return stored.snapshot(), nil
}

func (s *ObjectStore) SetLegalHold(ctx context.Context, ref ports.ObjectRef, hold ports.LegalHold) (ports.HoldReceipt, error) {
	address, err := s.locate(ctx, ref)
	if err != nil {
		return ports.HoldReceipt{}, err
	}
	if hold.On && hold.Reason == "" {
		return ports.HoldReceipt{}, ports.ErrLegalHoldReasonRequired
	}
	now := s.bucket.clock.Now()

	if err := s.bucket.acquire(ctx); err != nil {
		return ports.HoldReceipt{}, err
	}
	defer s.bucket.mu.Unlock()
	stored, ok := s.bucket.versions[address]
	if !ok {
		return ports.HoldReceipt{}, ports.ErrObjectNotFound
	}
	stored.info.LegalHold = hold.On
	return ports.HoldReceipt{Ref: ref, On: hold.On, Reason: hold.Reason, At: now}, nil
}

func (s *ObjectStore) Purge(ctx context.Context, ref ports.ObjectRef) (ports.PurgeReceipt, error) {
	address, err := s.locate(ctx, ref)
	if err != nil {
		return ports.PurgeReceipt{}, err
	}
	now := s.bucket.clock.Now()

	if err := s.bucket.acquire(ctx); err != nil {
		return ports.PurgeReceipt{}, err
	}
	defer s.bucket.mu.Unlock()
	stored, ok := s.bucket.versions[address]
	if !ok {
		return ports.PurgeReceipt{}, ports.ErrObjectNotFound
	}
	if stored.info.LegalHold {
		return ports.PurgeReceipt{}, ports.ErrObjectLocked
	}
	var retainedUntil ports.Instant
	if retention := stored.info.Retention; retention != nil {
		if retention.Until > now {
			return ports.PurgeReceipt{}, ports.ErrObjectLocked
		}
		retainedUntil = retention.Until
	}
	delete(s.bucket.versions, address)
	return ports.PurgeReceipt{Ref: ref, PurgedAt: now, RetainedUntil: retainedUntil}, nil
}

func (s *ObjectStore) checkRetention(now ports.Instant, r ports.Retention) error {
	if s.decl.Ceiling == nil {
		return ports.ErrRetentionCeilingUndeclared
	}
	if !r.Mode.Known() {
		return fmt.Errorf("%w: %q", ports.ErrRetentionModeUnknown, r.Mode)
	}
	if r.Until <= now {
		return ports.ErrRetentionUnbounded
	}
	if ceiling, ok := s.decl.Ceiling.Span.Add(now); ok && r.Until > ceiling {
		return ports.ErrRetentionAboveCeiling
	}
	return nil
}

// WHY: S3 keeps the retention date to the millisecond at most; dropping the
// fraction here makes a caller that compares nanoseconds fail on the fake too.
func wholeSeconds(r ports.Retention) ports.Retention {
	r.Until -= r.Until % retentionResolution
	return r
}

func (s *ObjectStore) locate(ctx context.Context, ref ports.ObjectRef) (objectAddress, error) {
	if ref.Version == "" {
		return objectAddress{}, ports.ErrVersionRequired
	}
	if err := ref.Key.Validate(); err != nil {
		return objectAddress{}, err
	}
	physical, err := s.physical(ctx, ref.Key)
	if err != nil {
		return objectAddress{}, err
	}
	return objectAddress{physical: physical, version: ref.Version}, nil
}

// WHY: the physical key is built only from the carrier, so another tenant's or
// scope's reference cannot resolve (IDN-14). The tenant/ segment keeps every
// tenant id, "platform" included, out of the platform/ prefix (IDN-19, IDN-20).
func (s *ObjectStore) physical(ctx context.Context, key ports.ObjectKey) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	execution, err := ports.RequireExecutionContext(ctx)
	if err != nil {
		return "", err
	}
	tenant, scoped := execution.Tenant()
	if s.decl.Scope == ports.ScopePlatform {
		if scoped {
			return "", ports.ErrScopeMismatch
		}
		return "platform/" + string(key), nil
	}
	if !scoped || strings.Contains(string(tenant), "/") {
		return "", ErrTenantUnresolved
	}
	return "tenant/" + string(tenant) + "/" + string(key), nil
}

func (v *storedVersion) snapshot() ports.ObjectInfo {
	info := v.info
	if v.info.Retention != nil {
		dated := *v.info.Retention
		info.Retention = &dated
	}
	return info
}
