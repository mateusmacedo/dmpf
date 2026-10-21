package application_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	alphanumeric = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	base64url    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
)

// WHY: the depguard of this block covers its tests and denies crypto/* and
// hash, so the digest is a hand-written FNV-1a over four lanes.
func fnvDigest(canonical []byte) ports.Fingerprint {
	var f ports.Fingerprint
	for lane := range 4 {
		h := uint64(14695981039346656037) ^ uint64(lane)
		for _, b := range canonical {
			h ^= uint64(b)
			h *= 1099511628211
		}
		for i := range 8 {
			f[lane*8+i] = byte(h >> (56 - 8*i))
		}
	}
	return f
}

func fixedDigest(fill func(i int) byte, seen *[]byte) func([]byte) ports.Fingerprint {
	return func(canonical []byte) ports.Fingerprint {
		if seen != nil {
			*seen = append([]byte(nil), canonical...)
		}
		var f ports.Fingerprint
		for i := range f {
			f[i] = fill(i)
		}
		return f
	}
}

func sampleIntent() usecase.OutboundIntent {
	return usecase.OutboundIntent{Tenant: "t-1", Destination: "psp", Operation: "charge", IntentID: "i-9", Generation: 2}
}

func profileWith(alphabet string, maxLength int) ports.ExternalProfile {
	p := validProfile()
	p.KeyAlphabet, p.KeyMaxLength = alphabet, maxLength
	return p
}

func TestTheOutboundKeyKeepsItsCanonicalEncoding(t *testing.T) {
	var seen []byte
	policy := usecase.OutboundKeyPolicy{Digest: fixedDigest(func(int) byte { return 0 }, &seen)}

	if _, err := usecase.DeriveOutboundKey(policy, validProfile(), sampleIntent()); err != nil {
		t.Fatalf("DeriveOutboundKey = %v", err)
	}

	var golden []byte
	golden = append(golden, 's', 17)
	golden = append(golden, "dmpf.outbound-key"...)
	golden = append(golden, 's', 3, 't', '-', '1')
	golden = append(golden, 's', 3, 'p', 's', 'p')
	golden = append(golden, 's', 6, 'c', 'h', 'a', 'r', 'g', 'e')
	golden = append(golden, 's', 3, 'i', '-', '9')
	golden = append(golden, 'i', 1, 0x04)
	if !bytes.Equal(seen, golden) {
		t.Fatalf("canonical = %x, want %x: a resend after a release must carry the first attempt's key", seen, golden)
	}
}

func TestTheOutboundKeyReadsTheDigestAsBitsInTheAlphabet(t *testing.T) {
	ascending := fixedDigest(func(i int) byte { return byte(i) }, nil)
	allOnes := fixedDigest(func(int) byte { return 0xFF }, nil)
	tests := []struct {
		name    string
		digest  func([]byte) ports.Fingerprint
		profile ports.ExternalProfile
		want    string
	}{
		{"hex reads every nibble", ascending, profileWith(hexAlphabet, 64),
			"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"},
		{"length capped by the profile", ascending, profileWith(hexAlphabet, 40),
			"000102030405060708090a0b0c0d0e0f10111213"},
		{"length capped by the digest", ascending, profileWith(hexAlphabet, 100),
			"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"},
		{"62 symbols index only the first 32, without modulo", allOnes, profileWith(alphanumeric, 60),
			strings.Repeat("V", 51)},
		{"base64url reads 6 bits", fixedDigest(func(i int) byte { return []byte{0x00, 0x10, 0x83}[i%3] }, nil), profileWith(base64url, 42),
			strings.Repeat("ABCD", 10) + "AB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := usecase.DeriveOutboundKey(usecase.OutboundKeyPolicy{Digest: tt.digest}, tt.profile, sampleIntent())
			if err != nil {
				t.Fatalf("DeriveOutboundKey = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("key = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestANonPowerOfTwoAlphabetNeverReachesItsTail(t *testing.T) {
	policy := usecase.OutboundKeyPolicy{Digest: fnvDigest}
	for generation := range int64(64) {
		intent := sampleIntent()
		intent.Generation = generation
		key, err := usecase.DeriveOutboundKey(policy, profileWith(alphanumeric, 60), intent)
		if err != nil {
			t.Fatalf("DeriveOutboundKey = %v", err)
		}
		if len(key) != 51 {
			t.Fatalf("len = %d, want 51 symbols of 5 bits", len(key))
		}
		if i := strings.IndexFunc(string(key), func(r rune) bool { return !strings.ContainsRune(alphanumeric[:32], r) }); i >= 0 {
			t.Fatalf("key %q uses %q, outside the first 32 symbols: the reading is biased", key, key[i])
		}
	}
}

func TestTheOutboundKeyIsStableForTheSameIntent(t *testing.T) {
	policy := usecase.OutboundKeyPolicy{Digest: fnvDigest}
	first, err := usecase.DeriveOutboundKey(policy, validProfile(), sampleIntent())
	if err != nil {
		t.Fatalf("DeriveOutboundKey = %v", err)
	}
	again, err := usecase.DeriveOutboundKey(policy, validProfile(), sampleIntent())
	if err != nil {
		t.Fatalf("DeriveOutboundKey = %v", err)
	}
	if first != again {
		t.Fatalf("the same intent derives %q and %q", first, again)
	}
}

func TestEveryFieldOfTheIntentChangesTheOutboundKey(t *testing.T) {
	policy := usecase.OutboundKeyPolicy{Digest: fnvDigest}
	base, err := usecase.DeriveOutboundKey(policy, validProfile(), sampleIntent())
	if err != nil {
		t.Fatalf("DeriveOutboundKey = %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*usecase.OutboundIntent)
	}{
		{"tenant", func(i *usecase.OutboundIntent) { i.Tenant = "t-2" }},
		{"destination", func(i *usecase.OutboundIntent) { i.Destination = "bank" }},
		{"operation", func(i *usecase.OutboundIntent) { i.Operation = "refund" }},
		{"intent id", func(i *usecase.OutboundIntent) { i.IntentID = "i-10" }},
		{"generation", func(i *usecase.OutboundIntent) { i.Generation = 3 }},
		{"field boundary", func(i *usecase.OutboundIntent) { i.Tenant, i.Destination = "t-1p", "sp" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := sampleIntent()
			tt.mutate(&intent)
			got, err := usecase.DeriveOutboundKey(policy, validProfile(), intent)
			if err != nil {
				t.Fatalf("DeriveOutboundKey = %v", err)
			}
			if got == base {
				t.Fatalf("changing the %s kept the key %q", tt.name, got)
			}
		})
	}
}

func TestTheOutboundKeyRefusesAnIncompleteDerivation(t *testing.T) {
	invalid := validProfile()
	invalid.KeyMaxLength = 31
	tests := []struct {
		name    string
		policy  usecase.OutboundKeyPolicy
		profile ports.ExternalProfile
		want    error
	}{
		{"no digest", usecase.OutboundKeyPolicy{}, validProfile(), usecase.ErrIncompleteOutboundPolicy},
		{"profile below 128 bits", usecase.OutboundKeyPolicy{Digest: fnvDigest}, invalid, usecase.ErrInvalidExternalProfile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := usecase.DeriveOutboundKey(tt.policy, tt.profile, sampleIntent())
			if !errors.Is(err, tt.want) {
				t.Fatalf("DeriveOutboundKey = %v, want %v", err, tt.want)
			}
			if key != "" {
				t.Fatalf("key = %q alongside a refusal, want empty", key)
			}
		})
	}
}

func TestTheOutboundKeyRefusesAnIncompleteIntent(t *testing.T) {
	policy := usecase.OutboundKeyPolicy{Digest: fnvDigest}
	tests := []struct {
		name   string
		field  string
		mutate func(*usecase.OutboundIntent)
	}{
		{"empty intent id", "intent id", func(i *usecase.OutboundIntent) { i.IntentID = "" }},
		{"empty destination", "destination", func(i *usecase.OutboundIntent) { i.Destination = "" }},
		{"empty operation", "operation", func(i *usecase.OutboundIntent) { i.Operation = "" }},
		{"negative generation", "generation", func(i *usecase.OutboundIntent) { i.Generation = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := sampleIntent()
			tt.mutate(&intent)
			key, err := usecase.DeriveOutboundKey(policy, validProfile(), intent)
			if !errors.Is(err, usecase.ErrIncompleteOutboundIntent) {
				t.Fatalf("DeriveOutboundKey = %v, want ErrIncompleteOutboundIntent", err)
			}
			if !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("error %q does not name the %s", err, tt.field)
			}
			if key != "" {
				t.Fatalf("key = %q alongside a refusal, want empty", key)
			}
		})
	}
}

func TestTheOutboundKeyAcceptsAnIntentWithoutTenant(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*usecase.OutboundIntent)
	}{
		{"complete intent", func(*usecase.OutboundIntent) {}},
		{"platform operation without tenant", func(i *usecase.OutboundIntent) { i.Tenant = "" }},
		{"first generation", func(i *usecase.OutboundIntent) { i.Generation = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent := sampleIntent()
			tt.mutate(&intent)
			key, err := usecase.DeriveOutboundKey(usecase.OutboundKeyPolicy{Digest: fnvDigest}, validProfile(), intent)
			if err != nil {
				t.Fatalf("DeriveOutboundKey = %v, want accepted", err)
			}
			if key == "" {
				t.Fatal("accepted intent derived an empty key")
			}
		})
	}
}
