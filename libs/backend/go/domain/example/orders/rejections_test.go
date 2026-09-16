package orders_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
)

func TestDeclaredCodesAreValid(t *testing.T) {
	codes := map[string]domain.Code{
		"CodeItemLimitExceeded": orders.CodeItemLimitExceeded,
		"CodeEmptyOrder":        orders.CodeEmptyOrder,
		"CodeOrderNotOpen":      orders.CodeOrderNotOpen,
	}
	for name, code := range codes {
		t.Run(name, func(t *testing.T) {
			if !code.Valid() {
				t.Fatalf("%s = %q is not a valid context/reason code", name, code)
			}
		})
	}
}
