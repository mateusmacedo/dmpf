package application

import (
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	minKeyBits     = 128
	firstKeySymbol = 0x21
	lastKeySymbol  = 0x7E
)

// ErrInvalidExternalProfile is the refusal of a partner profile no command can
// run under; the key floor is the 128 bits a drawn idempotency key already has.
var ErrInvalidExternalProfile = errors.New("application: external profile invalid")

// ValidateExternalProfile is the check NextStep presumes and DeriveOutboundKey runs.
func ValidateExternalProfile(p ports.ExternalProfile) error {
	if p.KeySemantics < ports.KeyNative || p.KeySemantics > ports.KeyNone {
		return fmt.Errorf("%w: key semantics %d undeclared", ErrInvalidExternalProfile, p.KeySemantics)
	}
	if p.Efficacy < ports.EfficacyOnSubmit || p.Efficacy > ports.EfficacyByObservation {
		return fmt.Errorf("%w: efficacy %d undeclared", ErrInvalidExternalProfile, p.Efficacy)
	}
	if p.Quiescence < 0 {
		return fmt.Errorf("%w: negative quiescence %d", ErrInvalidExternalProfile, p.Quiescence)
	}
	if p.MaxQueries <= 0 {
		return fmt.Errorf("%w: query ceiling %d not positive", ErrInvalidExternalProfile, p.MaxQueries)
	}
	if err := validateKeyAlphabet(p.KeyAlphabet); err != nil {
		return err
	}
	bits := symbolBits(p.KeyAlphabet)
	if minLength := (minKeyBits + bits - 1) / bits; p.KeyMaxLength < minLength {
		return fmt.Errorf("%w: key of %d symbols of %d bits is below %d bits", ErrInvalidExternalProfile, p.KeyMaxLength, bits, minKeyBits)
	}
	return nil
}

func validateKeyAlphabet(alphabet string) error {
	var seen [lastKeySymbol + 1]bool
	for i := range len(alphabet) {
		b := alphabet[i]
		if b < firstKeySymbol || b > lastKeySymbol {
			return fmt.Errorf("%w: alphabet byte 0x%02x is not visible ASCII", ErrInvalidExternalProfile, b)
		}
		if seen[b] {
			return fmt.Errorf("%w: alphabet repeats %q", ErrInvalidExternalProfile, b)
		}
		seen[b] = true
	}
	if symbolBits(alphabet) == 0 {
		return fmt.Errorf("%w: alphabet of %d symbols carries no bit", ErrInvalidExternalProfile, len(alphabet))
	}
	return nil
}

func symbolBits(alphabet string) int {
	bits := 0
	for n := len(alphabet); n > 1; n >>= 1 {
		bits++
	}
	return bits
}
