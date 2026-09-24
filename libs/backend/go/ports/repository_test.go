package ports_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestCrossTenantAccessAnswersAsNotFound(t *testing.T) {
	access := ports.CrossTenantAccess{Object: "orders/o-1", ContextTenant: "globex", DataTenant: "acme"}
	wrapped := fmt.Errorf("application: find order o-1: %w", access)

	if !errors.Is(wrapped, ports.ErrNotFound) {
		t.Fatalf("errors.Is(%v, ErrNotFound) = false; every caller that answers not-found must keep answering it", wrapped)
	}
	if access.Error() != ports.ErrNotFound.Error() {
		t.Fatalf("Error() = %q, want %q: the text reaches responses and logs, and must not name the owner", access.Error(), ports.ErrNotFound.Error())
	}

	var got ports.CrossTenantAccess
	if !errors.As(wrapped, &got) || got != access {
		t.Fatalf("errors.As() = %+v, want %+v: the internal record has to tell the two cases apart", got, access)
	}
}
