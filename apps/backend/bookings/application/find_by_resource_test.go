package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type crossTenantResource struct{ access ports.CrossTenantAccess }

func (r crossTenantResource) LoadByResource(context.Context, domain.ResourceID) ([]domain.BookingSnapshot, error) {
	return nil, r.access
}

type capturedResult struct{ results []ports.Result }

func (c *capturedResult) BeginOperation(ctx context.Context, _ string) (context.Context, ports.EndOperation) {
	return ctx, func(r ports.Result) { c.results = append(c.results, r) }
}

func (c *capturedResult) Audit(context.Context, ports.AuditEvent) {}

// IDN-13 and IDN-12 on the relation: the caller of a resource held only by
// another tenant gets the empty answer a resource nobody holds gets, and the
// instrumentation still receives the access to record it.
func TestFindByResourceOfAnotherTenantAnswersEmptyAndReportsTheAccess(t *testing.T) {
	h := newHarness(t)
	access := ports.CrossTenantAccess{Object: "bookings_booking?resource_id=r-1", ContextTenant: "globex", DataTenant: "acme"}
	h.service.ResourceReader = crossTenantResource{access: access}
	captured := &capturedResult{}
	h.service.Instrumentation = captured

	snapshots, err := h.service.FindBookingByResource(withExecution(t, context.Background()), "r-1")

	if err != nil || len(snapshots) != 0 {
		t.Fatalf("FindBookingByResource() = %v, %v; want an empty answer, like a resource nobody holds", snapshots, err)
	}
	var got ports.CrossTenantAccess
	if len(captured.results) != 1 || !errors.As(captured.results[0].Err, &got) || got != access {
		t.Fatalf("EndOperation(%+v), want the access handed over for the security record", captured.results)
	}
}
