package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func TestDeclaredCodesAreValid(t *testing.T) {
	codes := map[string]kernel.Code{
		"CodeItemLimitExceeded": domain.CodeItemLimitExceeded,
		"CodeEmptyOrder":        domain.CodeEmptyOrder,
		"CodeOrderNotOpen":      domain.CodeOrderNotOpen,
	}
	for name, code := range codes {
		t.Run(name, func(t *testing.T) {
			if !code.Valid() {
				t.Fatalf("%s = %q is not a valid context/reason code", name, code)
			}
		})
	}
}
