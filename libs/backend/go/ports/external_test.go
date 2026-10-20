package ports_test

import (
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type namedValue struct {
	value fmt.Stringer
	want  string
}

func TestTheExternalVocabularyNamesEveryValue(t *testing.T) {
	tests := []struct {
		name   string
		values []namedValue
	}{
		{"class", []namedValue{
			{ports.ExternalClass(0), "invalid"},
			{ports.ClassAccepted, "accepted"},
			{ports.ClassRetryableTransport, "retryable_transport"},
			{ports.ClassRetryableUpstream, "retryable_upstream"},
			{ports.ClassFinalValidation, "final_validation"},
			{ports.ClassFinalBusiness, "final_business"},
			{ports.ClassFinalAuthorization, "final_authorization"},
			{ports.ClassUnknownSent, "unknown_sent"},
			{ports.ClassUnknownAmbiguous, "unknown_ambiguous"},
			{ports.ClassSchemaMismatch, "schema_mismatch"},
			{ports.ClassSchemaMismatch + 1, "invalid"},
		}},
		{"finding", []namedValue{
			{ports.Finding(0), "none"},
			{ports.FindingApplied, "applied"},
			{ports.FindingAbsent, "absent"},
			{ports.FindingRejected, "rejected"},
			{ports.FindingPending, "pending"},
			{ports.FindingInconclusive, "inconclusive"},
			{ports.FindingInconclusive + 1, "invalid"},
		}},
		{"key semantics", []namedValue{
			{ports.KeySemantics(0), "invalid"},
			{ports.KeyNative, "native"},
			{ports.KeyHeader, "header"},
			{ports.KeyNone, "none"},
			{ports.KeyNone + 1, "invalid"},
		}},
		{"efficacy", []namedValue{
			{ports.Efficacy(0), "invalid"},
			{ports.EfficacyOnSubmit, "on_submit"},
			{ports.EfficacyByObservation, "by_observation"},
			{ports.EfficacyByObservation + 1, "invalid"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, v := range tt.values {
				if got := v.value.String(); got != v.want {
					t.Errorf("%T(%d).String() = %q; want %q", v.value, v.value, got, v.want)
				}
			}
		})
	}
}

func TestTheZeroOfEveryExternalEnumIsNoDeclaredValue(t *testing.T) {
	if ports.ClassAccepted == 0 || ports.FindingApplied == 0 || ports.KeyNative == 0 || ports.EfficacyOnSubmit == 0 {
		t.Fatal("a declared value equals zero; an unset field would read as a decision")
	}
}

func TestTheExternalClassesAreNineDistinctValues(t *testing.T) {
	classes := []ports.ExternalClass{
		ports.ClassAccepted, ports.ClassRetryableTransport, ports.ClassRetryableUpstream,
		ports.ClassFinalValidation, ports.ClassFinalBusiness, ports.ClassFinalAuthorization,
		ports.ClassUnknownSent, ports.ClassUnknownAmbiguous, ports.ClassSchemaMismatch,
	}
	seen := map[string]bool{}
	for _, c := range classes {
		name := c.String()
		if name == "invalid" || seen[name] {
			t.Fatalf("class %d named %q is invalid or repeated", c, name)
		}
		seen[name] = true
	}
	if len(seen) != 9 {
		t.Fatalf("%d classes, want the nine of the outcome table", len(seen))
	}
}
