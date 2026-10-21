package application

import (
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	outboundKeyDomain = "dmpf.outbound-key"
	digestBits        = len(ports.Fingerprint{}) * 8
)

// ErrIncompleteOutboundPolicy is DeriveOutboundKey's refusal of a policy without
// a digest: without it, the key cannot be derived again for a resend (RES-34).
var ErrIncompleteOutboundPolicy = errors.New("application: outbound key policy lacks a digest")

// ErrIncompleteOutboundIntent is DeriveOutboundKey's refusal of an intent whose
// key would not single it out: two distinct commands would derive the same key,
// and a partner deduplicating on it would drop the second in silence (IDM-11).
var ErrIncompleteOutboundIntent = errors.New("application: outbound intent incomplete")

// OutboundKeyPolicy is fixed by the composition root, like IdempotencyPolicy.
// Digest must be cryptographically secure with at least 128 bits of strength;
// SHA-256 realizes it outside this block, which imports no crypto.
type OutboundKeyPolicy struct {
	Digest func(canonical []byte) ports.Fingerprint
}

// OutboundIntent is what the outbound key derives from; Tenant is empty for a
// platform operation. IntentID is minted once, when the intent commits; Generation
// advances only by the use case's decision, after a final rejection is corrected.
type OutboundIntent struct {
	Tenant      string
	Destination string
	Operation   string
	IntentID    string
	Generation  int64
}

// DeriveOutboundKey renders the intent's digest in the partner's alphabet using
// only its first 2^b symbols, so no symbol is favoured; it refuses a key below
// 128 bits.
func DeriveOutboundKey(policy OutboundKeyPolicy, profile ports.ExternalProfile, intent OutboundIntent) (ports.OutboundKey, error) {
	if err := validateOutboundIntent(intent); err != nil {
		return "", err
	}
	if policy.Digest == nil {
		return "", ErrIncompleteOutboundPolicy
	}
	if err := ValidateExternalProfile(profile); err != nil {
		return "", err
	}
	canonical := NewFingerprint(outboundKeyDomain).
		String(intent.Tenant).
		String(intent.Destination).
		String(intent.Operation).
		String(intent.IntentID).
		Int(intent.Generation).
		Canonical()
	digest := policy.Digest(canonical)

	bits := symbolBits(profile.KeyAlphabet)
	key := make([]byte, min(profile.KeyMaxLength, digestBits/bits))
	for i := range key {
		key[i] = profile.KeyAlphabet[readBits(digest, i*bits, bits)]
	}
	return ports.OutboundKey(key), nil
}

func validateOutboundIntent(intent OutboundIntent) error {
	switch {
	case intent.IntentID == "":
		return fmt.Errorf("%w: empty intent id", ErrIncompleteOutboundIntent)
	case intent.Destination == "":
		return fmt.Errorf("%w: empty destination", ErrIncompleteOutboundIntent)
	case intent.Operation == "":
		return fmt.Errorf("%w: empty operation", ErrIncompleteOutboundIntent)
	case intent.Generation < 0:
		return fmt.Errorf("%w: negative generation %d", ErrIncompleteOutboundIntent, intent.Generation)
	}
	return nil
}

func readBits(digest ports.Fingerprint, offset, width int) int {
	value := 0
	for bit := offset; bit < offset+width; bit++ {
		value = value<<1 | int(digest[bit/8]>>(7-bit%8)&1)
	}
	return value
}
