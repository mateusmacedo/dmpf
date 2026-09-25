package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type instrumentationRecorder struct {
	results []ports.Result
}

func (r *instrumentationRecorder) BeginOperation(ctx context.Context, _ string) (context.Context, ports.EndOperation) {
	return ctx, func(result ports.Result) { r.results = append(r.results, result) }
}

func (r *instrumentationRecorder) Audit(context.Context, ports.AuditEvent) {}

func instrumentedWith(t *testing.T, authorize func(context.Context, application.Operation) error) (*harness, *instrumentationRecorder) {
	t.Helper()
	h := newHarness(t)
	instr := &instrumentationRecorder{}
	h.service.Instrumentation = instr
	h.service.Authorize = authorize
	return h, instr
}

func TestReserveDeniedReportsDeniedWithoutTheError(t *testing.T) {
	h, instr := instrumentedWith(t, func(context.Context, application.Operation) error {
		return errors.Join(errors.New("policy engine refused"), ports.ErrDenied)
	})

	_, err := h.service.ReserveBooking(withExecution(t, context.Background()), application.ReserveBooking{BookingID: testBookingID, ResourceID: testResourceID, Quantity: 1})

	if !errors.Is(err, ports.ErrDenied) {
		t.Fatalf("ReserveBooking() error = %v, want a denial", err)
	}
	if len(instr.results) != 1 || instr.results[0].Outcome != ports.OutcomeDenied || instr.results[0].Err != nil {
		t.Fatalf("results = %+v, want one denied result without error: a denial is not a technical failure", instr.results)
	}
}

func TestReserveAuthorizerFailureReportsFailedNotDenied(t *testing.T) {
	failure := errors.New("policy engine unreachable")
	h, instr := instrumentedWith(t, func(context.Context, application.Operation) error { return failure })

	_, err := h.service.ReserveBooking(withExecution(t, context.Background()), application.ReserveBooking{BookingID: testBookingID, ResourceID: testResourceID, Quantity: 1})

	if !errors.Is(err, failure) {
		t.Fatalf("ReserveBooking() error = %v, want the authorizer failure", err)
	}
	if len(instr.results) != 1 || instr.results[0].Outcome != ports.OutcomeFailed || !errors.Is(instr.results[0].Err, failure) {
		t.Fatalf("results = %+v, want one failed result carrying the error", instr.results)
	}
}
