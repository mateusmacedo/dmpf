//go:build integration

package provider_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// This is what the memory provider declares it cannot prove and hands to
// KRN-06: two transactions that really run at the same time, over a real
// isolation level, with the loser told so instead of silently overwriting
// the winner.
func TestConcurrentSaveLetsExactlyOneWriterThrough(t *testing.T) {
	pool := appkit.OpenPool(t)
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
			results <- withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.OrderID, domain.Snapshot]) error {
				_, version, err := repo.Load(ctx, repoOrderID)
				if err != nil {
					return err
				}
				// Both read before either writes: without this barrier the
				// second transaction could start after the first committed and
				// never contend, which would prove nothing.
				loaded.Done()
				<-release
				return repo.Save(ctx, repoOrderID, snapshot(quantity), version)
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
		t.Fatalf("%d committed and %d conflicted, want 1 and %d — a lost update got through",
			committed, conflicted, writers-1)
	}
	if _, version := load(t, pool); version != 2 {
		t.Fatalf("version = %d, want 2 — exactly one write advanced the aggregate", version)
	}
}
