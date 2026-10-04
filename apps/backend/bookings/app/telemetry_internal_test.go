package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestClassifyGivesTheIdempotencySentinelsTheirCategory(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"key absent":     {ports.ErrIdempotencyKeyAbsent, string(application.Validation)},
		"key invalid":    {ports.ErrIdempotencyKeyInvalid, string(application.Validation)},
		"key reused":     {fmt.Errorf("bookings: %w", ports.ErrIdempotencyMismatch), string(application.Validation)},
		"key in flight":  {ports.ErrIdempotencyInFlight, string(application.Conflict)},
		"already exists": {fmt.Errorf("bookings: %w", ports.ErrAlreadyExists), string(application.Conflict)},
		"failure":        {application.NewFailure(application.NotFound, false, errors.New("bookings: gone")), string(application.NotFound)},
		"anything else":  {errors.New("bookings: boom"), "_OTHER"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := classify(c.err); got != c.want {
				t.Fatalf("classify(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}

func TestClassifyGivesTheRepositorySentinelsTheCategoryTheEdgeAnswers(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"not found":               {fmt.Errorf("bookings: %w", ports.ErrNotFound), string(application.NotFound)},
		"cross-tenant access":     {fmt.Errorf("bookings: %w", ports.CrossTenantAccess{Object: "bookings/x-1", ContextTenant: "acme", DataTenant: "globex"}), string(application.NotFound)},
		"version conflict":        {fmt.Errorf("bookings: %w", ports.ErrVersionConflict), string(application.Conflict)},
		"failure over a sentinel": {application.NewFailure(application.Unexpected, false, ports.ErrNotFound), string(application.Unexpected)},
	} {
		t.Run(name, func(t *testing.T) {
			if got := classify(c.err); got != c.want {
				t.Fatalf("classify(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}
