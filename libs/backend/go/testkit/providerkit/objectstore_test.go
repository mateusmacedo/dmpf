package providerkit_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	clauseIntegrityNoKey   = "put.integrity: a write refused for its hash leaves no key behind"
	clauseRetentionNoKey   = "retention.dated: a write refused for its retention leaves no key behind"
	clauseHoldAfterExpiry  = "hold.independent: the hold keeps Purge refused after the retention expired"
	clausePurgeReceipt     = "purge.receipt: a purge after the retention expired returns the receipt and the version is gone"
	clausePlatformPrefix   = "platform.scope: the platform store writes under platform/, a tenant under tenant/<id>/"
	clauseHeadStoredCipher = "head.state: Head reports the encryption the version was stored with, not the reading store's"
)

func TestMemoryObjectStoreConforms(t *testing.T) {
	v := providerkit.ObjectStore(providerkit.MemoryObjectStore)
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("the fake exercises every proof; nothing should be skipped: %v", v.Skipped)
	}
}

func TestObjectStoreSkipsByNameWhatTheSubjectCannotExercise(t *testing.T) {
	v := providerkit.ObjectStore(func() providerkit.ObjectStoreSubject {
		s := providerkit.MemoryObjectStore()
		s.Advance, s.PhysicalKeys, s.OtherEncryption = nil, nil, ports.Encryption{}
		return s
	})
	tb.Require(t, v)
	want := []string{clauseIntegrityNoKey, clauseRetentionNoKey, clauseHoldAfterExpiry, clausePurgeReceipt, clausePlatformPrefix, clauseHeadStoredCipher}
	if !slices.Equal(v.Skipped, want) {
		t.Fatalf("skipped = %v, want %v", v.Skipped, want)
	}
}

func TestObjectStoreRunsOnAShorterRetentionUnit(t *testing.T) {
	v := providerkit.ObjectStore(func() providerkit.ObjectStoreSubject {
		s := providerkit.MemoryObjectStore()
		s.RetentionUnit = ports.RetentionSpan(time.Minute)
		return s
	})
	tb.Require(t, v)
}

func TestObjectStoreRefusesARetentionUnitFinerThanTheSecond(t *testing.T) {
	v := providerkit.ObjectStore(func() providerkit.ObjectStoreSubject {
		s := providerkit.MemoryObjectStore()
		s.RetentionUnit = ports.RetentionSpan(1500 * time.Millisecond)
		return s
	})
	if v.OK() || len(v.Diagnostics) != 11 || !strings.Contains(v.Diagnostics[0].Detail, "RetentionUnit") {
		t.Fatalf("diagnostics = %v, want one RetentionUnit refusal per clause", v.Failures())
	}
}

type objectMutant func(decl ports.ObjectStoreDeclaration, store ports.ObjectStore) ports.ObjectStore

// mutated wraps every store the fake opens, so the kit meets a realization that
// breaks exactly one rule and must name the clause it broke.
func mutated(m objectMutant) func() providerkit.ObjectStoreSubject {
	return func() providerkit.ObjectStoreSubject {
		s := providerkit.MemoryObjectStore()
		open := s.Open
		s.Open = func(decl ports.ObjectStoreDeclaration) (ports.ObjectStore, error) {
			store, err := open(decl)
			if err != nil {
				return nil, err
			}
			return m(decl, store), nil
		}
		return s
	}
}

type lockIgnoring struct{ ports.ObjectStore }

func (s lockIgnoring) Purge(ctx context.Context, ref ports.ObjectRef) (ports.PurgeReceipt, error) {
	receipt, err := s.ObjectStore.Purge(ctx, ref)
	if errors.Is(err, ports.ErrObjectLocked) {
		return ports.PurgeReceipt{Ref: ref}, nil
	}
	return receipt, err
}

type downgrading struct{ ports.ObjectStore }

func (s downgrading) Retain(ctx context.Context, ref ports.ObjectRef, to ports.Retention) (ports.ObjectInfo, error) {
	info, err := s.ObjectStore.Retain(ctx, ref, to)
	if errors.Is(err, ports.ErrRetentionModeChange) {
		return info, nil
	}
	return info, err
}

type ceilingless struct{ ports.ObjectStore }

func (s ceilingless) Put(ctx context.Context, req ports.PutRequest) (ports.ObjectVersion, error) {
	version, err := s.ObjectStore.Put(ctx, req)
	if errors.Is(err, ports.ErrRetentionAboveCeiling) {
		return ports.ObjectVersion{Key: req.Key, Version: "forged"}, nil
	}
	return version, err
}

var errForbidden = errors.New("objectstore_test: forbidden")

type tellingTenants struct{ ports.ObjectStore }

func (s tellingTenants) Get(ctx context.Context, ref ports.ObjectRef) (ports.Object, error) {
	object, err := s.ObjectStore.Get(ctx, ref)
	if errors.Is(err, ports.ErrObjectNotFound) {
		return object, errForbidden
	}
	return object, err
}

type hashBlind struct{ ports.ObjectStore }

func (s hashBlind) Put(ctx context.Context, req ports.PutRequest) (ports.ObjectVersion, error) {
	version, err := s.ObjectStore.Put(ctx, req)
	if errors.Is(err, ports.ErrIntegrity) {
		return ports.ObjectVersion{Key: req.Key, Version: "unchecked"}, nil
	}
	return version, err
}

type reasonless struct{ ports.ObjectStore }

func (s reasonless) SetLegalHold(ctx context.Context, ref ports.ObjectRef, hold ports.LegalHold) (ports.HoldReceipt, error) {
	if hold.On && hold.Reason == "" {
		hold.Reason = "invented"
	}
	return s.ObjectStore.SetLegalHold(ctx, ref, hold)
}

type scopeBlindHold struct{ ports.ObjectStore }

func (s scopeBlindHold) SetLegalHold(ctx context.Context, ref ports.ObjectRef, hold ports.LegalHold) (ports.HoldReceipt, error) {
	receipt, err := s.ObjectStore.SetLegalHold(ctx, ref, hold)
	if errors.Is(err, ports.ErrScopeMismatch) {
		return ports.HoldReceipt{Ref: ref, On: hold.On, Reason: hold.Reason}, nil
	}
	return receipt, err
}

type leakyMiss struct{ ports.ObjectStore }

func (s leakyMiss) Head(ctx context.Context, ref ports.ObjectRef) (ports.ObjectInfo, error) {
	info, err := s.ObjectStore.Head(ctx, ref)
	if errors.Is(err, ports.ErrObjectNotFound) {
		info.Size = 42
	}
	return info, err
}

type echoingEncryption struct {
	ports.ObjectStore
	declared ports.Encryption
}

func (s echoingEncryption) Head(ctx context.Context, ref ports.ObjectRef) (ports.ObjectInfo, error) {
	info, err := s.ObjectStore.Head(ctx, ref)
	if err == nil {
		info.Encryption = s.declared
	}
	return info, err
}

type promotingPastCeiling struct{ ports.ObjectStore }

func (s promotingPastCeiling) Retain(ctx context.Context, ref ports.ObjectRef, to ports.Retention) (ports.ObjectInfo, error) {
	current, headErr := s.Head(ctx, ref)
	info, err := s.ObjectStore.Retain(ctx, ref, to)
	governed := headErr == nil && current.Retention != nil && current.Retention.Mode == ports.RetentionGovernance
	if errors.Is(err, ports.ErrRetentionAboveCeiling) && governed && to.Mode == ports.RetentionCompliance {
		current.Retention = &to
		return current, nil
	}
	return info, err
}

type frozenGovernance struct{ ports.ObjectStore }

func (s frozenGovernance) Retain(ctx context.Context, ref ports.ObjectRef, to ports.Retention) (ports.ObjectInfo, error) {
	if to.Mode == ports.RetentionGovernance {
		return ports.ObjectInfo{}, ports.ErrRetentionNotExtended
	}
	return s.ObjectStore.Retain(ctx, ref, to)
}

type writingThenRefusing struct{ ports.ObjectStore }

func (s writingThenRefusing) Put(ctx context.Context, req ports.PutRequest) (ports.ObjectVersion, error) {
	version, err := s.ObjectStore.Put(ctx, req)
	if errors.Is(err, ports.ErrIntegrity) {
		sum := sha256.Sum256(req.Body)
		req.ContentSHA256 = hex.EncodeToString(sum[:])
		_, _ = s.ObjectStore.Put(ctx, req)
		return ports.ObjectVersion{}, err
	}
	return version, err
}

type escapingKeys struct{ ports.ObjectStore }

func (s escapingKeys) Put(ctx context.Context, req ports.PutRequest) (ports.ObjectVersion, error) {
	version, err := s.ObjectStore.Put(ctx, req)
	if errors.Is(err, ports.ErrObjectKeyInvalid) {
		return ports.ObjectVersion{Key: req.Key, Version: "escaped"}, nil
	}
	return version, err
}

func unprefixedPlatform() providerkit.ObjectStoreSubject {
	s := providerkit.MemoryObjectStore()
	keys := s.PhysicalKeys
	s.PhysicalKeys = func() ([]string, error) {
		listed, err := keys()
		for i, key := range listed {
			listed[i] = strings.TrimPrefix(key, "platform/")
		}
		return listed, err
	}
	return s
}

func wrap[T ports.ObjectStore](build func(ports.ObjectStore) T) func() providerkit.ObjectStoreSubject {
	return mutated(func(_ ports.ObjectStoreDeclaration, store ports.ObjectStore) ports.ObjectStore { return build(store) })
}

func TestObjectStoreKitCatchesBrokenRealizations(t *testing.T) {
	tests := []struct {
		name    string
		subject func() providerkit.ObjectStoreSubject
		clause  string
	}{
		{"purge ignores the lock", wrap(func(s ports.ObjectStore) lockIgnoring { return lockIgnoring{s} }), "retention.compliance"},
		{"retain accepts a downgrade", wrap(func(s ports.ObjectStore) downgrading { return downgrading{s} }), "retention.governance"},
		{"retain promotes past the ceiling", wrap(func(s ports.ObjectStore) promotingPastCeiling { return promotingPastCeiling{s} }), "retention.governance"},
		{"retain never extends GOVERNANCE", wrap(func(s ports.ObjectStore) frozenGovernance { return frozenGovernance{s} }), "retention.governance"},
		{"put ignores the ceiling", wrap(func(s ports.ObjectStore) ceilingless { return ceilingless{s} }), "retention.dated"},
		{"put skips the hash", wrap(func(s ports.ObjectStore) hashBlind { return hashBlind{s} }), "put.integrity"},
		{"put writes before refusing the hash", wrap(func(s ports.ObjectStore) writingThenRefusing { return writingThenRefusing{s} }), "put.integrity"},
		{"get tells another tenant's object apart", wrap(func(s ports.ObjectStore) tellingTenants { return tellingTenants{s} }), "tenant.scope"},
		{"head leaks state with a miss", wrap(func(s ports.ObjectStore) leakyMiss { return leakyMiss{s} }), "tenant.scope"},
		{"put accepts an escaping key", wrap(func(s ports.ObjectStore) escapingKeys { return escapingKeys{s} }), "tenant.scope"},
		{"hold invents a reason", wrap(func(s ports.ObjectStore) reasonless { return reasonless{s} }), "hold.independent"},
		{"platform hold ignores the tenant", wrap(func(s ports.ObjectStore) scopeBlindHold { return scopeBlindHold{s} }), "platform.scope"},
		{"platform writes without its prefix", unprefixedPlatform, "platform.scope"},
		{"head echoes the declared encryption", mutated(func(decl ports.ObjectStoreDeclaration, s ports.ObjectStore) ports.ObjectStore {
			return echoingEncryption{ObjectStore: s, declared: decl.Encryption}
		}), "head.state"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := providerkit.ObjectStore(tt.subject)
			if v.OK() {
				t.Fatalf("the kit passed a store whose %s", tt.name)
			}
			if !slices.ContainsFunc(v.Diagnostics, func(d providerkit.Diagnostic) bool { return strings.HasPrefix(d.Clause, tt.clause+":") }) {
				t.Fatalf("no diagnostic under %s; got %v", tt.clause, v.Failures())
			}
		})
	}
}
