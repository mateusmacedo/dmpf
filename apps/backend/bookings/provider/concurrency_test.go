//go:build integration

package provider_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestConcurrentSaveLetsExactlyOneWriterThrough(t *testing.T) {
	pool := openPool(t)
	seed(t, pool)

	const writers = 2
	var (
		loaded  sync.WaitGroup
		release = make(chan struct{})
		results = make(chan error, writers)
		running sync.WaitGroup
	)
	loaded.Add(writers)
	running.Add(writers)

	for i := range writers {
		go func(quantity int) {
			defer running.Done()
			results <- withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.BookingID, domain.BookingSnapshot]) error {
				_, version, err := repo.Load(ctx, repoBookingID)
				if err != nil {
					return err
				}
				loaded.Done()
				<-release
				return repo.Save(ctx, repoBookingID, snap(quantity), version)
			})
		}(i + 10)
	}

	loaded.Wait()
	close(release)
	running.Wait()
	close(results)

	var committed, conflicted int
	for err := range results {
		switch {
		case err == nil:
			committed++
		case errors.Is(err, ports.ErrVersionConflict):
			conflicted++
		default:
			t.Fatalf("Within() = %v, want nil or ErrVersionConflict", err)
		}
	}

	if committed != 1 || conflicted != writers-1 {
		t.Fatalf("%d committed and %d conflicted, want 1 and %d",
			committed, conflicted, writers-1)
	}
	if _, version := loadFromPool(t, pool); version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}
}
