package memory_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	objectSecond = ports.RetentionSpan(1_000_000_000)
	objectDay    = 24 * 60 * 60 * objectSecond
	objectNow    = ports.Instant(1_000 * objectSecond)
)

func objectDeclaration(scope ports.Scope) ports.ObjectStoreDeclaration {
	return ports.ObjectStoreDeclaration{
		Scope:      scope,
		Encryption: ports.Encryption{Algorithm: "aws:kms", KeyRef: "alias/test"},
		Ceiling:    &ports.RetentionCeiling{Span: 400 * objectDay, Origin: "policy:test"},
	}
}

func newObjectStore(t *testing.T, bucket *memory.ObjectBucket, decl ports.ObjectStoreDeclaration) *memory.ObjectStore {
	t.Helper()
	store, err := memory.NewObjectStore(bucket, decl)
	if err != nil {
		t.Fatalf("NewObjectStore(%s) = %v", decl.Scope, err)
	}
	return store
}

func fixedBucket() *memory.ObjectBucket {
	return memory.NewObjectBucket(memory.FixedClock{At: objectNow})
}

func putRequest(key ports.ObjectKey, body []byte, retention *ports.Retention) ports.PutRequest {
	sum := sha256.Sum256(body)
	return ports.PutRequest{Key: key, Body: body, ContentSHA256: hex.EncodeToString(sum[:]), Retention: retention}
}

func putBody(t *testing.T, ctx context.Context, store *memory.ObjectStore, key ports.ObjectKey, body []byte, retention *ports.Retention) ports.ObjectVersion {
	t.Helper()
	version, err := store.Put(ctx, putRequest(key, body, retention))
	if err != nil {
		t.Fatalf("Put(%s) = %v", key, err)
	}
	return version
}

func TestObjectStoreScopesWriteUnderDisjointPrefixes(t *testing.T) {
	bucket := fixedBucket()
	tenant := newObjectStore(t, bucket, objectDeclaration(ports.ScopeTenant))
	platform := newObjectStore(t, bucket, objectDeclaration(ports.ScopePlatform))

	putBody(t, inTenant(t, "acme"), tenant, "seal", []byte("acme"), nil)
	putBody(t, inTenant(t, "platform"), tenant, "seal", []byte("tenant named platform"), nil)
	putBody(t, tenantlessCtx(t), platform, "seal", []byte("platform"), nil)

	want := []string{"platform/seal", "tenant/acme/seal", "tenant/platform/seal"}
	if got := bucket.PhysicalKeys(); !slices.Equal(got, want) {
		t.Fatalf("PhysicalKeys() = %v, want %v", got, want)
	}
}

func TestNewObjectStoreRefusesAnIncompleteConstruction(t *testing.T) {
	noScope := objectDeclaration("")
	noEncryption := objectDeclaration(ports.ScopeTenant)
	noEncryption.Encryption = ports.Encryption{}
	tests := []struct {
		name   string
		bucket *memory.ObjectBucket
		decl   ports.ObjectStoreDeclaration
		want   error
	}{
		{"no scope", fixedBucket(), noScope, ports.ErrScopeUndeclared},
		{"no encryption", fixedBucket(), noEncryption, ports.ErrEncryptionUndeclared},
		{"no bucket", nil, objectDeclaration(ports.ScopeTenant), memory.ErrIncompleteObjectStore},
		{"a bucket without a clock", memory.NewObjectBucket(nil), objectDeclaration(ports.ScopeTenant), memory.ErrIncompleteObjectStore},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := memory.NewObjectStore(tt.bucket, tt.decl)
			if !errors.Is(err, tt.want) || store != nil {
				t.Fatalf("NewObjectStore() = %v, %v; want no store and %v", store, err, tt.want)
			}
		})
	}
}

func TestObjectStoreKeepsItsOwnCopyOfTheBody(t *testing.T) {
	store := newObjectStore(t, fixedBucket(), objectDeclaration(ports.ScopeTenant))
	ctx := inTenant(t, "acme")
	body := []byte("evidence")
	version := putBody(t, ctx, store, "evidence", body, nil)

	body[0] = 'X'
	first, err := store.Get(ctx, version.Ref())
	if err != nil || string(first.Body) != "evidence" {
		t.Fatalf("Get() after mutating the written slice = %q, %v; want %q", first.Body, err, "evidence")
	}
	first.Body[0] = 'Y'
	second, err := store.Get(ctx, version.Ref())
	if err != nil || string(second.Body) != "evidence" {
		t.Fatalf("Get() after mutating a read slice = %q, %v; want %q", second.Body, err, "evidence")
	}
}

func TestObjectStoreKeepsItsOwnCopyOfTheDeclaration(t *testing.T) {
	decl := objectDeclaration(ports.ScopeTenant)
	store := newObjectStore(t, fixedBucket(), decl)
	decl.Ceiling.Span = 4_000 * objectDay

	beyond := ports.Retention{Mode: ports.RetentionGovernance, Until: objectNow + ports.Instant(800*objectDay)}
	_, err := store.Put(inTenant(t, "acme"), putRequest("seal", []byte("seal"), &beyond))
	if !errors.Is(err, ports.ErrRetentionAboveCeiling) {
		t.Fatalf("Put(past the declared ceiling, after the caller widened its copy) = %v, want ErrRetentionAboveCeiling", err)
	}
}

func TestObjectStoreRetentionReadIsACopy(t *testing.T) {
	store := newObjectStore(t, fixedBucket(), objectDeclaration(ports.ScopeTenant))
	ctx := inTenant(t, "acme")
	held := ports.Retention{Mode: ports.RetentionCompliance, Until: objectNow + ports.Instant(10*objectDay)}
	version := putBody(t, ctx, store, "seal", []byte("seal"), &held)

	info, err := store.Head(ctx, version.Ref())
	if err != nil {
		t.Fatalf("Head() = %v", err)
	}
	info.Retention.Until = objectNow + 1
	again, err := store.Head(ctx, version.Ref())
	if err != nil || again.Retention == nil || *again.Retention != held {
		t.Fatalf("Head() after mutating a read retention = %+v, %v; want %+v", again.Retention, err, held)
	}
}

func TestObjectStoreKeepsRetentionToTheSecond(t *testing.T) {
	store := newObjectStore(t, fixedBucket(), objectDeclaration(ports.ScopeTenant))
	ctx := inTenant(t, "acme")
	whole := objectNow + ports.Instant(10*objectDay)
	version := putBody(t, ctx, store, "seal", []byte("seal"), &ports.Retention{Mode: ports.RetentionGovernance, Until: whole + 999_999_999})

	info, err := store.Head(ctx, version.Ref())
	if err != nil || info.Retention == nil || info.Retention.Until != whole {
		t.Fatalf("Head() = %+v, %v; want Until %d, the fraction of the second dropped as S3 does", info.Retention, err, whole)
	}
	if _, err := store.Retain(ctx, version.Ref(), ports.Retention{Mode: ports.RetentionGovernance, Until: whole + 500_000_000}); !errors.Is(err, ports.ErrRetentionNotExtended) {
		t.Fatalf("Retain(a later fraction of the same second) = %v, want ErrRetentionNotExtended", err)
	}
}

func TestObjectStoreRefusesACancelledContext(t *testing.T) {
	bucket := fixedBucket()
	store := newObjectStore(t, bucket, objectDeclaration(ports.ScopeTenant))
	acme := inTenant(t, "acme")
	version := putBody(t, acme, store, "seal", []byte("seal"), nil)
	cancelled, cancel := context.WithCancel(acme)
	cancel()

	_, putErr := store.Put(cancelled, putRequest("other", []byte("other"), nil))
	_, getErr := store.Get(cancelled, version.Ref())
	_, headErr := store.Head(cancelled, version.Ref())
	_, retainErr := store.Retain(cancelled, version.Ref(), ports.Retention{Mode: ports.RetentionGovernance, Until: objectNow + ports.Instant(objectDay)})
	_, holdErr := store.SetLegalHold(cancelled, version.Ref(), ports.LegalHold{On: true, Reason: "audit"})
	_, purgeErr := store.Purge(cancelled, version.Ref())
	for i, err := range []error{putErr, getErr, headErr, retainErr, holdErr, purgeErr} {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("operation %d on a cancelled context = %v, want context.Canceled", i, err)
		}
	}
	if info, err := store.Head(acme, version.Ref()); err != nil || info.Retention != nil || info.LegalHold {
		t.Fatalf("Head() = %+v, %v; want the version untouched by the cancelled calls", info, err)
	}
	if got := bucket.PhysicalKeys(); !slices.Equal(got, []string{"tenant/acme/seal"}) {
		t.Fatalf("PhysicalKeys() = %v, want only the first write", got)
	}
}

// The first ctx.Err() check happens before waiting for the bucket's lock, so a
// caller whose context dies while queued has to be refused again when its turn comes.
func TestAContextCancelledWhileWaitingNeverReachesTheBucket(t *testing.T) {
	bucket := fixedBucket()
	store := newObjectStore(t, bucket, objectDeclaration(ports.ScopeTenant))
	ctx := &cancelledAfterFirstCheck{Context: inTenant(t, "acme")}

	_, err := store.Put(ctx, putRequest("queued", []byte("queued"), nil))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Put() = %v, want context.Canceled", err)
	}
	if got := ctx.checks.Load(); got != 2 {
		t.Fatalf("ctx.Err() checked %d times, want 2 — the refusal must come after taking the lock", got)
	}
	if got := bucket.PhysicalKeys(); len(got) != 0 {
		t.Fatalf("PhysicalKeys() = %v, want none", got)
	}
}

func TestObjectStoreServesConcurrentCallers(t *testing.T) {
	bucket := fixedBucket()
	tenant := newObjectStore(t, bucket, objectDeclaration(ports.ScopeTenant))
	platform := newObjectStore(t, bucket, objectDeclaration(ports.ScopePlatform))
	acme, seal := inTenant(t, "acme"), tenantlessCtx(t)

	const callers = 16
	var wg sync.WaitGroup
	errs := make(chan error, 2*callers)
	for i := range callers {
		wg.Go(func() {
			key := ports.ObjectKey(fmt.Sprintf("evidence/%d", i))
			version, err := tenant.Put(acme, putRequest(key, []byte(key), nil))
			if err == nil {
				_, err = tenant.SetLegalHold(acme, version.Ref(), ports.LegalHold{On: true, Reason: "audit"})
			}
			if err == nil {
				_, err = tenant.SetLegalHold(acme, version.Ref(), ports.LegalHold{})
			}
			if err == nil {
				_, err = tenant.Purge(acme, version.Ref())
			}
			errs <- err
		})
		wg.Go(func() {
			key := ports.ObjectKey(fmt.Sprintf("seal/%d", i))
			retention := ports.Retention{Mode: ports.RetentionGovernance, Until: objectNow + ports.Instant(objectDay)}
			version, err := platform.Put(seal, putRequest(key, []byte(key), &retention))
			if err == nil {
				_, err = platform.Retain(seal, version.Ref(), ports.Retention{Mode: ports.RetentionCompliance, Until: retention.Until})
			}
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("a concurrent caller failed: %v", err)
		}
	}
	keys := bucket.PhysicalKeys()
	if len(keys) != callers || !slices.ContainsFunc(keys, func(k string) bool { return k == "platform/seal/0" }) {
		t.Fatalf("PhysicalKeys() = %v, want the %d platform seals and no tenant evidence left", keys, callers)
	}
}
