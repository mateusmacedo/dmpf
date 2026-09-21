//go:build integration

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	httpedge "github.com/mateusmacedo/dmpf/apps/backend/bookings/app/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"

	"github.com/jackc/pgx/v5/pgxpool"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
)

type fixedClock struct{}

func (fixedClock) Now() ports.Instant { return 1_755_432_000 }

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

func newMux(pool *pgxpool.Pool) *http.ServeMux {
	bind := func(tx *postgres.Tx) application.Resources {
		return application.Resources{
			Bookings:  provider.NewBookingRepository(tx),
			Resources: provider.NewResourceRepository(tx),
			Outbox:    tx.Outbox(provider.Mapper{}),
		}
	}
	service := application.Service{
		UoW:            postgres.NewUnitOfWork(pool, bind),
		Reader:         provider.NewBookingReader(pool),
		ResourceReader: provider.NewBookingsByResourceReader(pool),
		Clock:          fixedClock{},
		IDs:            &sequenceIDs{},
		Authorize:      usecase.AllowAllWithContext[application.Operation](),
	}
	return httpedge.Mux(service, e2eBudget)
}

// WHY: the suite exercises the mux the process serves, middleware included, so
// a route that only answers without the edge's time policy fails here.
var e2eBudget = deadline.Budget{
	Dependency:        "postgres",
	Method:            "route",
	Limit:             5 * time.Second,
	Slack:             200 * time.Millisecond,
	EstimatedDuration: 200 * time.Millisecond,
}

func TestReserveBookingHTTPEndToEnd(t *testing.T) {
	pool := pg.OpenPool(t, "bookings_booking", "bookings_resource")
	mux := newMux(pool)

	body, _ := json.Marshal(map[string]any{"bookingId": "http-b-001", "resourceId": "http-r-001", "quantity": 3})
	req := httptest.NewRequest(http.MethodPost, "/bookings/booking", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-001")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["bookingId"] != "http-b-001" {
		t.Fatalf("bookingId = %q, want http-b-001", resp["bookingId"])
	}

	t.Run("outbox has a row", func(t *testing.T) {
		var count int
		err := pool.QueryRow(context.Background(), "SELECT count(*) FROM dmpf_outbox").Scan(&count)
		if err != nil {
			t.Fatalf("count outbox: %v", err)
		}
		if count != 1 {
			t.Fatalf("outbox count = %d, want 1", count)
		}
	})

	t.Run("find booking by id via HTTP", func(t *testing.T) {
		findReq := httptest.NewRequest(http.MethodGet, "/bookings/booking/http-b-001", nil)
		findRec := httptest.NewRecorder()
		mux.ServeHTTP(findRec, findReq)

		if findRec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", findRec.Code, findRec.Body.String())
		}
		var view map[string]any
		if err := json.NewDecoder(findRec.Body).Decode(&view); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if view["id"] != "http-b-001" {
			t.Fatalf("id = %v, want http-b-001", view["id"])
		}
		if view["status"] != "reserved" {
			t.Fatalf("status = %v, want reserved", view["status"])
		}
	})

	t.Run("cancel via HTTP", func(t *testing.T) {
		cancelReq := httptest.NewRequest(http.MethodPost, "/bookings/booking/http-b-001/cancel", nil)
		cancelReq.Header.Set("Idempotency-Key", "idem-002")
		cancelRec := httptest.NewRecorder()
		mux.ServeHTTP(cancelRec, cancelReq)

		if cancelRec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", cancelRec.Code, cancelRec.Body.String())
		}
	})

	t.Run("find by resource via HTTP", func(t *testing.T) {
		findReq := httptest.NewRequest(http.MethodGet, "/bookings/booking?resourceId=http-r-001", nil)
		findRec := httptest.NewRecorder()
		mux.ServeHTTP(findRec, findReq)

		if findRec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", findRec.Code, findRec.Body.String())
		}
		var views []map[string]any
		if err := json.NewDecoder(findRec.Body).Decode(&views); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(views) != 1 {
			t.Fatalf("len = %d, want 1", len(views))
		}
		if views[0]["status"] != "cancelled" {
			t.Fatalf("status = %v, want cancelled", views[0]["status"])
		}
	})
}

func TestReserveBookingHTTPRejectsInvalidQuantity(t *testing.T) {
	pool := pg.OpenPool(t, "bookings_booking", "bookings_resource")
	mux := newMux(pool)

	body, _ := json.Marshal(map[string]any{"bookingId": "http-b-002", "resourceId": "http-r-002", "quantity": 0})
	req := httptest.NewRequest(http.MethodPost, "/bookings/booking", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-003")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}
