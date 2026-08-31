package dmpfdomain

import (
	"testing"
)

func TestDmpfDomain(t *testing.T) {
	result := DmpfDomain("works")
	if result != "DmpfDomain works" {
		t.Error("Expected DmpfDomain to append 'works'")
	}
}
