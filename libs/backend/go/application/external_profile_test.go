package application_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const hexAlphabet = "0123456789abcdef"

func validProfile() ports.ExternalProfile {
	return ports.ExternalProfile{
		KeySemantics: ports.KeyNative,
		Efficacy:     ports.EfficacyOnSubmit,
		Quiescence:   1_000_000_000,
		MaxQueries:   3,
		KeyAlphabet:  hexAlphabet,
		KeyMaxLength: 32,
	}
}

func TestAValidExternalProfileIsAccepted(t *testing.T) {
	if err := usecase.ValidateExternalProfile(validProfile()); err != nil {
		t.Fatalf("ValidateExternalProfile(valid) = %v, want nil", err)
	}
}

func TestAnInvalidExternalProfileIsRefused(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ports.ExternalProfile)
	}{
		{"zero query ceiling", func(p *ports.ExternalProfile) { p.MaxQueries = 0 }},
		{"negative query ceiling", func(p *ports.ExternalProfile) { p.MaxQueries = -1 }},
		{"empty alphabet", func(p *ports.ExternalProfile) { p.KeyAlphabet = "" }},
		{"single-symbol alphabet", func(p *ports.ExternalProfile) { p.KeyAlphabet = "a"; p.KeyMaxLength = 1000 }},
		{"repeated byte", func(p *ports.ExternalProfile) { p.KeyAlphabet = "0123456789abcdee" }},
		{"space is not visible", func(p *ports.ExternalProfile) { p.KeyAlphabet = " 123456789abcdef" }},
		{"DEL is not visible", func(p *ports.ExternalProfile) { p.KeyAlphabet = "\x7f123456789abcdef" }},
		{"non-ASCII byte", func(p *ports.ExternalProfile) { p.KeyAlphabet = "é123456789abcd" }},
		{"length one short of 128 bits", func(p *ports.ExternalProfile) { p.KeyMaxLength = 31 }},
		{"zero length", func(p *ports.ExternalProfile) { p.KeyMaxLength = 0 }},
		{"no key semantics", func(p *ports.ExternalProfile) { p.KeySemantics = 0 }},
		{"key semantics out of range", func(p *ports.ExternalProfile) { p.KeySemantics = ports.KeyNone + 1 }},
		{"no efficacy", func(p *ports.ExternalProfile) { p.Efficacy = 0 }},
		{"efficacy out of range", func(p *ports.ExternalProfile) { p.Efficacy = ports.EfficacyByObservation + 1 }},
		{"negative quiescence", func(p *ports.ExternalProfile) { p.Quiescence = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.mutate(&p)
			err := usecase.ValidateExternalProfile(p)
			if !errors.Is(err, usecase.ErrInvalidExternalProfile) {
				t.Fatalf("ValidateExternalProfile = %v, want ErrInvalidExternalProfile", err)
			}
		})
	}
}

func TestTheProfileAcceptsTheExactly128BitBoundary(t *testing.T) {
	tests := []struct {
		name     string
		alphabet string
		length   int
	}{
		{"hex, 32 symbols of 4 bits", hexAlphabet, 32},
		{"base32, 26 symbols of 5 bits reach 130", "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567", 26},
		{"62 symbols read as 5 bits", alphanumeric, 26},
		{"base64url, 22 symbols of 6 bits reach 132", base64url, 22},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			p.KeyAlphabet, p.KeyMaxLength = tt.alphabet, tt.length
			if err := usecase.ValidateExternalProfile(p); err != nil {
				t.Fatalf("length %d = %v, want accepted", tt.length, err)
			}
			p.KeyMaxLength--
			if err := usecase.ValidateExternalProfile(p); !errors.Is(err, usecase.ErrInvalidExternalProfile) {
				t.Fatalf("length %d = %v, want refused below 128 bits", p.KeyMaxLength, err)
			}
		})
	}
}

func TestAKeyLengthAtTheTopOfIntIsAccepted(t *testing.T) {
	tests := []struct {
		name     string
		alphabet string
	}{
		{"two symbols of 1 bit", "01"},
		{"hex, 4 bits", hexAlphabet},
		{"base64url, 6 bits", base64url},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			p.KeyAlphabet, p.KeyMaxLength = tt.alphabet, math.MaxInt
			if err := usecase.ValidateExternalProfile(p); err != nil {
				t.Fatalf("length math.MaxInt = %v, want accepted", err)
			}
		})
	}
}

func TestAnAlphabetByteOutsideVisibleASCIIIsNamedSo(t *testing.T) {
	tests := []struct {
		name     string
		alphabet string
	}{
		{"space", " 123456789abcdef"},
		{"DEL", "\x7f123456789abcdef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			p.KeyAlphabet = tt.alphabet
			err := usecase.ValidateExternalProfile(p)
			if err == nil || !strings.Contains(err.Error(), "is not visible ASCII") {
				t.Fatalf("ValidateExternalProfile = %v, want a byte that is not visible ASCII", err)
			}
		})
	}
}
