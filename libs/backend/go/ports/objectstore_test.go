package ports_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const day = ports.RetentionSpan(24 * 60 * 60 * 1_000_000_000)

func validDeclaration() ports.ObjectStoreDeclaration {
	return ports.ObjectStoreDeclaration{
		Scope:      ports.ScopeTenant,
		Encryption: ports.Encryption{Algorithm: "aws:kms", KeyRef: "alias/evidence"},
		Ceiling:    &ports.RetentionCeiling{Span: 400 * day, Origin: "policy:evidence-retention"},
	}
}

func TestObjectStoreDeclarationValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ports.ObjectStoreDeclaration)
		want   error
	}{
		{"tenant scope with a ceiling", func(*ports.ObjectStoreDeclaration) {}, nil},
		{"platform scope", func(d *ports.ObjectStoreDeclaration) { d.Scope = ports.ScopePlatform }, nil},
		{"no ceiling", func(d *ports.ObjectStoreDeclaration) { d.Ceiling = nil }, nil},
		{"encryption without a key reference", func(d *ports.ObjectStoreDeclaration) { d.Encryption.KeyRef = "" }, nil},
		{"scope absent", func(d *ports.ObjectStoreDeclaration) { d.Scope = "" }, ports.ErrScopeUndeclared},
		{"scope unknown", func(d *ports.ObjectStoreDeclaration) { d.Scope = "global" }, ports.ErrScopeUndeclared},
		{"encryption absent", func(d *ports.ObjectStoreDeclaration) { d.Encryption = ports.Encryption{} }, ports.ErrEncryptionUndeclared},
		{"encryption with a key but no algorithm", func(d *ports.ObjectStoreDeclaration) { d.Encryption.Algorithm = "" }, ports.ErrEncryptionUndeclared},
		{"ceiling with a zero span", func(d *ports.ObjectStoreDeclaration) { d.Ceiling.Span = 0 }, ports.ErrRetentionCeilingUndeclared},
		{"ceiling with a negative span", func(d *ports.ObjectStoreDeclaration) { d.Ceiling.Span = -day }, ports.ErrRetentionCeilingUndeclared},
		{"ceiling without an origin", func(d *ports.ObjectStoreDeclaration) { d.Ceiling.Origin = "" }, ports.ErrRetentionCeilingUndeclared},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := validDeclaration()
			tt.mutate(&d)
			err := d.Validate()
			if tt.want == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("Validate() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRetentionSpanAdd(t *testing.T) {
	tests := []struct {
		name   string
		span   ports.RetentionSpan
		at     ports.Instant
		want   ports.Instant
		wantOK bool
	}{
		{"positive span", 2 * day, 10, ports.Instant(10 + 2*day), true},
		{"zero span", 0, 10, 10, true},
		{"negative span", -day, ports.Instant(3 * day), ports.Instant(2 * day), true},
		{"sum at the upper bound", 1, math.MaxInt64 - 1, math.MaxInt64, true},
		{"overflow past the upper bound", 1, math.MaxInt64, 0, false},
		{"overflow past the lower bound", -1, math.MinInt64, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.span.Add(tt.at)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("RetentionSpan(%d).Add(%d) = (%d, %v), want (%d, %v)", tt.span, tt.at, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestObjectNotFoundReadsAsNotFound(t *testing.T) {
	if !errors.Is(ports.ErrObjectNotFound, ports.ErrNotFound) {
		t.Fatal("errors.Is(ErrObjectNotFound, ErrNotFound) = false, want true (IDN-13)")
	}
	wrapped := fmt.Errorf("provider: %w", ports.ErrObjectNotFound)
	if !errors.Is(wrapped, ports.ErrObjectNotFound) || !errors.Is(wrapped, ports.ErrNotFound) {
		t.Fatalf("a wrapped ErrObjectNotFound must still read as both sentinels: %v", wrapped)
	}
	if errors.Is(ports.ErrNotFound, ports.ErrObjectNotFound) {
		t.Fatal("errors.Is(ErrNotFound, ErrObjectNotFound) = true, want false: an aggregate miss is not an object miss")
	}
}

func TestObjectVersionRefCarriesTheVersion(t *testing.T) {
	v := ports.ObjectVersion{Key: "seal/2026-11-01", Version: "v-1", ContentSHA256: "ab", Size: 2, StoredAt: 7}
	if got, want := v.Ref(), (ports.ObjectRef{Key: "seal/2026-11-01", Version: "v-1"}); got != want {
		t.Fatalf("Ref() = %+v, want %+v", got, want)
	}
}

type onlySixOperations struct{}

func (onlySixOperations) Put(context.Context, ports.PutRequest) (ports.ObjectVersion, error) {
	return ports.ObjectVersion{}, nil
}

func (onlySixOperations) Get(context.Context, ports.ObjectRef) (ports.Object, error) {
	return ports.Object{}, nil
}

func (onlySixOperations) Head(context.Context, ports.ObjectRef) (ports.ObjectInfo, error) {
	return ports.ObjectInfo{}, nil
}

func (onlySixOperations) Retain(context.Context, ports.ObjectRef, ports.Retention) (ports.ObjectInfo, error) {
	return ports.ObjectInfo{}, nil
}

func (onlySixOperations) SetLegalHold(context.Context, ports.ObjectRef, ports.LegalHold) (ports.HoldReceipt, error) {
	return ports.HoldReceipt{}, nil
}

func (onlySixOperations) Purge(context.Context, ports.ObjectRef) (ports.PurgeReceipt, error) {
	return ports.PurgeReceipt{}, nil
}

// WHY: a seventh method on ObjectStore — a delete without a version, a
// shortening, a bypass — stops this file from compiling (DAT-11).
var _ ports.ObjectStore = onlySixOperations{}

func TestObjectKeyValidate(t *testing.T) {
	tests := []struct {
		key   ports.ObjectKey
		valid bool
	}{
		{"seal", true},
		{"seal/2026-11-01", true},
		{"a/..b/c.", true},
		{"", false},
		{"/seal", false},
		{".", false},
		{"..", false},
		{"a/./b", false},
		{"a/../b", false},
		{"a/..", false},
	}
	for _, tt := range tests {
		err := tt.key.Validate()
		if tt.valid && err != nil {
			t.Errorf("ObjectKey(%q).Validate() = %v, want nil", tt.key, err)
		}
		if !tt.valid && !errors.Is(err, ports.ErrObjectKeyInvalid) {
			t.Errorf("ObjectKey(%q).Validate() = %v, want ErrObjectKeyInvalid", tt.key, err)
		}
	}
}

func TestObjectKeyValidateNeverEchoesTheKey(t *testing.T) {
	for _, key := range []ports.ObjectKey{"/cpf/12345678900", "cpf/../12345678900"} {
		err := key.Validate()
		if !errors.Is(err, ports.ErrObjectKeyInvalid) || strings.Contains(err.Error(), "12345678900") {
			t.Errorf("ObjectKey(%q).Validate() = %v, want ErrObjectKeyInvalid without the key in the message (DAT-22)", key, err)
		}
	}
}

func TestRetentionModeKnown(t *testing.T) {
	tests := []struct {
		mode ports.RetentionMode
		want bool
	}{
		{ports.RetentionCompliance, true},
		{ports.RetentionGovernance, true},
		{"", false},
		{"compliance", false},
	}
	for _, tt := range tests {
		if got := tt.mode.Known(); got != tt.want {
			t.Errorf("RetentionMode(%q).Known() = %v, want %v", tt.mode, got, tt.want)
		}
	}
}
