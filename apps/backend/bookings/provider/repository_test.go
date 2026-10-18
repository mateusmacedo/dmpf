//go:build integration

package provider_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const repoBookingID = domain.BookingID("b-1001")

func snap(quantity int) domain.BookingSnapshot {
	return domain.BookingSnapshot{
		ID:         repoBookingID,
		ResourceID: "r-200",
		Quantity:   quantity,
		Status:     domain.Reserved,
		ReservedAt: 1755432000,
	}
}

func TestBookingRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.BookingID, domain.BookingSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.BookingID, domain.BookingSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, ports.Repository[domain.BookingID, domain.BookingSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewBookingRepository, fn)
			},
			Reader:           provider.NewBookingReader(postgres.NewReadPool(pool)),
			NewID:            func(n int) domain.BookingID { return domain.BookingID("kit-" + strconv.Itoa(n)) },
			NewState:         snap,
			Marker:           func(s domain.BookingSnapshot) int { return s.Quantity },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	requireNothingSkipped(t, v)
}

func TestResourceRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.ResourceCode, domain.ResourceSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.ResourceCode, domain.ResourceSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, ports.Repository[domain.ResourceCode, domain.ResourceSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewResourceRepository, fn)
			},
			Reader: resourceReader{pool: pool},
			NewID:  func(n int) domain.ResourceCode { return domain.ResourceCode("kit-" + strconv.Itoa(n)) },
			NewState: func(marker int) domain.ResourceSnapshot {
				return domain.ResourceSnapshot{Name: domain.ResourceName("kit-name-" + strconv.Itoa(marker)), RegisteredAt: kernel.Instant(marker)}
			},
			Marker:           func(s domain.ResourceSnapshot) int { return int(s.RegisteredAt) },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	requireNothingSkipped(t, v)
}

func TestBookingRoundTripsTheWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewBookingReader(postgres.NewReadPool(pool))

	pg.Seed(t, ctx, pool, provider.NewBookingRepository, repoBookingID, snap(5))
	if got, version, err := reader.Load(ctx, repoBookingID); err != nil || version != 1 || !got.Equal(snap(5)) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v1", got, version, err, snap(5))
	}

	cancelled := snap(10)
	cancelled.Status = domain.Cancelled
	err := pg.Within(ctx, pool, provider.NewBookingRepository, func(ctx context.Context, repo ports.Repository[domain.BookingID, domain.BookingSnapshot]) error {
		return repo.Save(ctx, repoBookingID, cancelled, 1)
	})
	if err != nil {
		t.Fatalf("Save(update) = %v", err)
	}
	if got, version, err := reader.Load(ctx, repoBookingID); err != nil || version != 2 || !got.Equal(cancelled) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v2", got, version, err, cancelled)
	}
}

func TestResourceRoundTripsTheWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	want := domain.ResourceSnapshot{Code: "room-1", Name: "Room 1", RegisteredAt: 1_755_432_000_000_000_001}

	pg.Seed(t, ctx, pool, provider.NewResourceRepository, want.Code, want)

	if got, version, err := (resourceReader{pool: pool}).Load(ctx, want.Code); err != nil || version != 1 || !got.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v1", got, version, err, want)
	}
}

func TestResourceRepositoryHonoursTheUniqueKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Unique(func() providerkit.UniqueSubject[domain.ResourceCode, domain.ResourceSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.UniqueSubject[domain.ResourceCode, domain.ResourceSnapshot]{
			Within: func(ctx context.Context, fn func(ctx context.Context, w providerkit.UniqueWrites[domain.ResourceCode, domain.ResourceSnapshot]) error) error {
				uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) *postgres.Tx { return tx })
				return uow.Within(ctx, func(ctx context.Context, tx *postgres.Tx) error {
					return fn(ctx, providerkit.UniqueWrites[domain.ResourceCode, domain.ResourceSnapshot]{
						Repo: provider.NewResourceRepository(tx),
						Enqueue: func(ctx context.Context, n int) error {
							return tx.Outbox(provider.Mapper{}).Enqueue(ctx, resourceRegisteredEntry(n))
						},
					})
				})
			},
			Reader: resourceReader{pool: pool},
			NewID:  func(n int) domain.ResourceCode { return domain.ResourceCode("kit-" + strconv.Itoa(n)) },
			NewState: func(marker int, keys [][]string) domain.ResourceSnapshot {
				return domain.ResourceSnapshot{Name: domain.ResourceName(keys[0][0]), RegisteredAt: kernel.Instant(marker)}
			},
			Marker:      func(s domain.ResourceSnapshot) int { return int(s.RegisteredAt) },
			Table:       "resources",
			Constraints: []providerkit.UniqueConstraint{{Name: "resources_tenant_id_name_key", Columns: []string{"tenant_id", "name"}, Code: domain.CodeResourceDuplicateName}},
			Concurrent:  true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 3 {
		t.Fatalf("skipped = %v, want only the clauses that need a composite key or a second constraint", v.Skipped)
	}
}

func resourceRegisteredEntry(n int) ports.OutboxEntry {
	code := domain.ResourceCode("kit-event-" + strconv.Itoa(n))
	return ports.OutboxEntry{
		MessageID: ports.MessageID("evt-" + string(code)), OccurredAt: 1,
		Intent:        ports.PublishIntent{Destination: "bookings.events", PartitionKey: string(code)},
		AggregateType: "bookings.Resource", AggregateID: string(code), AggregateVersion: 1,
		Event: domain.ResourceRegistered{Code: code, Name: "Kit event", At: 1},
	}
}

func requireNothingSkipped(t *testing.T, v providerkit.Verdict) {
	t.Helper()
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

// resourceReader reads through the repository in a transaction of its own: the
// provider exposes no reader of resources, because no query reads one.
type resourceReader struct{ pool *pgxpool.Pool }

func (r resourceReader) Load(ctx context.Context, code domain.ResourceCode) (domain.ResourceSnapshot, ports.Version, error) {
	var (
		state   domain.ResourceSnapshot
		version ports.Version
	)
	err := pg.Within(ctx, r.pool, provider.NewResourceRepository, func(ctx context.Context, repo ports.Repository[domain.ResourceCode, domain.ResourceSnapshot]) error {
		var err error
		state, version, err = repo.Load(ctx, code)
		return err
	})
	return state, version, err
}
