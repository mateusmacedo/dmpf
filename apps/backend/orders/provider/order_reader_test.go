//go:build integration

package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func TestNewReaderLoadsWhatTheRepositoryWrote(t *testing.T) {
	pool := appkit.OpenPool(t)
	seed(t, pool)

	loaded, version, err := provider.NewOrderReader(postgres.NewReadPool(pool)).Load(withExecution(t, context.Background()), repoOrderID)

	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
	if !loaded.Equal(snapshot(1)) {
		t.Fatalf("snapshot = %+v, want %+v", loaded, snapshot(1))
	}
}

func TestNewReaderReportsAnAbsentOrder(t *testing.T) {
	pool := appkit.OpenPool(t)

	_, _, err := provider.NewOrderReader(postgres.NewReadPool(pool)).Load(withExecution(t, context.Background()), "o-absent")

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound", err)
	}
}
