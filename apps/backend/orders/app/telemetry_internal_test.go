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
		"key reused":     {fmt.Errorf("orders: %w", ports.ErrIdempotencyMismatch), string(application.Validation)},
		"key in flight":  {ports.ErrIdempotencyInFlight, string(application.Conflict)},
		"already exists": {fmt.Errorf("orders: %w", ports.ErrAlreadyExists), string(application.Conflict)},
		"failure":        {application.NewFailure(application.NotFound, false, errors.New("orders: gone")), string(application.NotFound)},
		"anything else":  {errors.New("orders: boom"), "unclassified"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := classify(c.err); got != c.want {
				t.Fatalf("classify(%v) = %q, want %q", c.err, got, c.want)
			}
		})
	}
}
