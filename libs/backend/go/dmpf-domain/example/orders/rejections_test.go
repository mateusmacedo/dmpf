package orders_test

import (
	"testing"

	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
)

func TestDeclaredCodesAreValid(t *testing.T) {
	codes := map[string]dmpfdomain.Code{
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
