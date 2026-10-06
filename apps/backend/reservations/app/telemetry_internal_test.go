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
		"key reused":     {fmt.Errorf("reservations: %w", ports.ErrIdempotencyMismatch), string(application.Validation)},
		"key in flight":  {ports.ErrIdempotencyInFlight, string(application.Conflict)},
		"already exists": {fmt.Errorf("reservations: %w", ports.ErrAlreadyExists), string(application.Conflict)},
		"failure":        {application.NewFailure(application.NotFound, false, errors.New("reservations: gone")), string(application.NotFound)},
		"anything else":  {errors.New("reservations: boom"), "_OTHER"},
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
		"not found":               {fmt.Errorf("reservations: %w", ports.ErrNotFound), string(application.NotFound)},
		"cross-tenant access":     {fmt.Errorf("reservations: %w", ports.CrossTenantAccess{Object: "reservations/x-1", ContextTenant: "acme", DataTenant: "globex"}), string(application.NotFound)},
		"version conflict":        {fmt.Errorf("reservations: %w", ports.ErrVersionConflict), string(application.Conflict)},
		"failure over a sentinel": {application.NewFailure(application.Unexpected, false, ports.ErrNotFound), string(application.Unexpected)},
	} {
		t.Run(name, func(t *testing.T) {
			if got := classify(c.err); got != c.want {
				t.Fatalf("classify(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}
