//go:build integration

package provider_test

import (
	"context"
	"fmt"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/provider"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const e2eOccurred = ports.Instant(1_755_432_000_000_000_000)

// WHY: provider → application é célula proibida, mas _test.go fica fora do
// universo do verificador e a composition root é justamente o papel que um
// teste encena. Nenhum arquivo de produção deste módulo alcança o bloco.

type fixedClock struct{}

func (fixedClock) Now() ports.Instant { return e2eOccurred }

type sequenceIDs struct {
	mu     sync.Mutex
	issued int
}

func (g *sequenceIDs) NewMessageID() ports.MessageID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.issued++
	return ports.MessageID(fmt.Sprintf("m-%06d", g.issued))
}

// outboxRow is the committed row as the relay would read it.
type outboxRow struct {
	MessageType      string
	SchemaVersion    string
	AggregateVersion int64
	Destination      string
	Status           string
}

func newService(pool *pgxpool.Pool) application.Service {
	bind := func(tx *postgres.Tx) application.Resources {
		return application.Resources{
			Orders: provider.NewRepository(tx),
			Outbox: tx.Outbox(provider.Mapper{}),
		}
	}
	return application.Service{
		UoW: postgres.NewUnitOfWork(pool, bind),
		// Reader stays nil: it serves FindOrder (UOW-11), and no clause here
		// queries outside a transaction.
		Clock:     fixedClock{},
		IDs:       &sequenceIDs{},
		Authorize: usecase.AllowAll[application.Operation](),
		ItemLimit: 3,
	}
}

// The order of the two commands is fixed by the aggregate, not by preference:
// PlaceOrder only loads, and Place refuses an order with no items, so AddItem
// is what creates the order and OrderPlaced can only come second.
func TestTheUseCaseRunsEndToEndOverPostgres(t *testing.T) {
	pool := pg.OpenPool(t)
	service := newService(pool)
	ctx := context.Background()

	added, err := service.AddItem(withExecution(t, ctx), application.AddItem{Order: repoOrderID, SKU: "sku-1", Quantity: 2})
	if err != nil {
		t.Fatalf("AddItem() = %v, want nil", err)
	}
	if rejection, refused := added.Rejection(); refused {
		t.Fatalf("AddItem() was rejected with %v, want accepted", rejection)
	}

	t.Run("the item added row lands at version 1", func(t *testing.T) {
		if _, version := load(t, pool); version != 1 {
			t.Fatalf("version = %d, want 1", version)
		}
		row := outboxRowOf(t, pool, "m-000001")
		want := outboxRow{
			MessageType:      "com.company.orders.item-added.v1",
			SchemaVersion:    "type.googleapis.com/company.orders.event.v1.ItemAdded",
			AggregateVersion: 1,
			Destination:      "orders.events",
			Status:           "pending",
		}
		if row != want {
			t.Fatalf("outbox row = %+v, want %+v", row, want)
		}
	})

	placed, err := service.PlaceOrder(withExecution(t, ctx), application.PlaceOrder{Order: repoOrderID})
	if err != nil {
		t.Fatalf("PlaceOrder() = %v, want nil", err)
	}
	if rejection, refused := placed.Rejection(); refused {
		t.Fatalf("PlaceOrder() was rejected with %v, want accepted", rejection)
	}

	t.Run("the order placed row lands at version 2", func(t *testing.T) {
		if _, version := load(t, pool); version != 2 {
			t.Fatalf("version = %d, want 2", version)
		}
		row := outboxRowOf(t, pool, "m-000002")
		want := outboxRow{
			MessageType:      "com.company.orders.order-placed.v1",
			SchemaVersion:    "type.googleapis.com/company.orders.event.v1.OrderPlaced",
			AggregateVersion: 2,
			Destination:      "orders.events",
			Status:           "pending",
		}
		if row != want {
			t.Fatalf("outbox row = %+v, want %+v", row, want)
		}
	})

	t.Run("a rejection commits nothing and is not an error", func(t *testing.T) {
		ordersBefore, outboxBefore := counts(t, pool)

		outcome, err := service.AddItem(withExecution(t, ctx), application.AddItem{Order: repoOrderID, SKU: "sku-2", Quantity: 1})

		if err != nil {
			t.Fatalf("AddItem() on a placed order = %v, want nil — a refusal is not a technical failure (DEC-04)", err)
		}
		rejection, refused := outcome.Rejection()
		if !refused {
			t.Fatal("AddItem() on a placed order was accepted, want rejected")
		}
		if rejection.Code() != domain.CodeOrderNotOpen {
			t.Errorf("rejection code = %q, want %q", rejection.Code(), domain.CodeOrderNotOpen)
		}

		ordersAfter, outboxAfter := counts(t, pool)
		if ordersAfter != ordersBefore || outboxAfter != outboxBefore {
			t.Fatalf("counts moved on a rejection: orders %d→%d, outbox %d→%d (UOW-06)",
				ordersBefore, ordersAfter, outboxBefore, outboxAfter)
		}
	})
}

func outboxRowOf(t *testing.T, pool *pgxpool.Pool, messageID string) outboxRow {
	t.Helper()

	var row outboxRow
	err := pool.QueryRow(context.Background(), `
		SELECT message_type, schema_version, aggregate_version, destination, status
		FROM outbox WHERE message_id = $1`, messageID).Scan(
		&row.MessageType, &row.SchemaVersion, &row.AggregateVersion, &row.Destination, &row.Status)
	if err != nil {
		t.Fatalf("SELECT outbox row %s = %v, want nil", messageID, err)
	}
	return row
}

func counts(t *testing.T, pool *pgxpool.Pool) (int, int) {
	t.Helper()

	var ordersCount, outboxCount int
	err := pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM dmpf_example_orders), (SELECT count(*) FROM outbox)`).
		Scan(&ordersCount, &outboxCount)
	if err != nil {
		t.Fatalf("counts = %v, want nil", err)
	}
	return ordersCount, outboxCount
}
