//go:build integration

package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

func TestNewReaderLoadsWhatTheRepositoryWrote(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	pg.Seed(t, ctx, pool, provider.NewReservationRepository, repoOrderID, snapshot(5))
	want, wantVersion := pg.Load(t, ctx, pool, provider.NewReservationRepository, repoOrderID)

	got, version, err := provider.NewReservationReader(postgres.NewReadPool(pool)).Load(withExecution(t, context.Background()), repoOrderID)

	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if version != wantVersion || !got.Equal(want) {
		t.Fatalf("Load() = %+v v%d, want %+v v%d", got, version, want, wantVersion)
	}
}

func TestNewReaderReportsAnAbsentReservation(t *testing.T) {
	pool := appkit.OpenPool(t)

	_, _, err := provider.NewReservationReader(postgres.NewReadPool(pool)).Load(withExecution(t, context.Background()), "o-absent")

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound", err)
	}
}
