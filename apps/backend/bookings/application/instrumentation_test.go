package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type instrumentationRecorder struct {
	results []ports.Result
	audits  []ports.AuditEvent
}

func (r *instrumentationRecorder) BeginOperation(ctx context.Context, _ string) (context.Context, ports.EndOperation) {
	return ctx, func(result ports.Result) { r.results = append(r.results, result) }
}

func (r *instrumentationRecorder) Audit(_ context.Context, event ports.AuditEvent) {
	r.audits = append(r.audits, event)
}

func (r *instrumentationRecorder) requireOnlyAudit(t *testing.T, want ports.AuditEvent) {
	t.Helper()
	if len(r.audits) != 1 || r.audits[0] != want {
		t.Fatalf("audits = %+v, want exactly %+v", r.audits, want)
	}
}

func newInstrumentedHarness(t *testing.T) (*harness, *instrumentationRecorder) {
	t.Helper()
	h := newHarness(t)
	instr := &instrumentationRecorder{}
	h.service.Instrumentation = instr
	return h, instr
}

func instrumentedWith(t *testing.T, authorize func(context.Context, application.Operation) error) (*harness, *instrumentationRecorder) {
	t.Helper()
	h, instr := newInstrumentedHarness(t)
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

func TestReserveAcceptedAuditsOnce(t *testing.T) {
	h, instr := newInstrumentedHarness(t)

	if _, err := h.service.ReserveBooking(withExecution(t, context.Background()), reserve(5)); err != nil {
		t.Fatalf("ReserveBooking() error = %v, want nil", err)
	}

	instr.requireOnlyAudit(t, ports.AuditEvent{
		Object:  string(testBookingID),
		Action:  application.OperationReserveBooking,
		Outcome: ports.OutcomeAccepted,
		At:      testOccurred,
	})
}

func TestCancelAcceptedAuditsOnce(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	h.seedBooking(t, domain.BookingSnapshot{
		ID: testBookingID, ResourceID: testResourceID, Quantity: 5,
		Status: domain.Reserved, ReservedAt: 1000,
	}, 1)

	if _, err := h.service.CancelBooking(withExecution(t, context.Background()), application.CancelBooking{BookingID: testBookingID}); err != nil {
		t.Fatalf("CancelBooking() error = %v, want nil", err)
	}

	instr.requireOnlyAudit(t, ports.AuditEvent{
		Object:  string(testBookingID),
		Action:  application.OperationCancelBooking,
		Outcome: ports.OutcomeAccepted,
		At:      testOccurred,
	})
}

func TestRegisterAcceptedAuditsOnce(t *testing.T) {
	h, instr := newInstrumentedHarness(t)

	if _, err := h.service.RegisterResource(withExecution(t, context.Background()), application.RegisterResource{Code: testResCode}); err != nil {
		t.Fatalf("RegisterResource() error = %v, want nil", err)
	}

	instr.requireOnlyAudit(t, ports.AuditEvent{
		Object:  string(testResCode),
		Action:  application.OperationRegisterResource,
		Outcome: ports.OutcomeAccepted,
		At:      testOccurred,
	})
}
