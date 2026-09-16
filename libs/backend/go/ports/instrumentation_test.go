package ports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestNoInstrumentationKeepsTheContextAndEndsInertly(t *testing.T) {
	type key struct{}
	parent := context.WithValue(context.Background(), key{}, "marker")

	ctx, end := ports.NoInstrumentation().BeginOperation(parent, "orders.AddItem")

	if ctx.Value(key{}) != "marker" {
		t.Fatal("BeginOperation dropped the parent context; the inert hook must not alter propagation")
	}
	end(ports.Result{Outcome: ports.OutcomeAccepted})
}

func TestNoInstrumentationAuditIsInert(t *testing.T) {
	ports.NoInstrumentation().Audit(context.Background(), ports.AuditEvent{
		Object:  "order-1",
		Action:  "orders.AddItem",
		Outcome: ports.OutcomeAccepted,
		At:      ports.Instant(1),
	})
}

func TestResultCarriesTheRawErrorForTheProviderToClassify(t *testing.T) {
	cause := errors.New("connection reset by peer")

	result := ports.Result{Outcome: ports.OutcomeFailed, Err: cause}

	if !errors.Is(result.Err, cause) {
		t.Fatal("Result.Err must carry the raw error: the taxonomy of MET-10 belongs to the provider")
	}
}

func TestResultOnAcceptedCarriesNoError(t *testing.T) {
	result := ports.Result{Outcome: ports.OutcomeAccepted}

	if result.Err != nil {
		t.Fatalf("Result.Err = %v on the accepting branch, want nil", result.Err)
	}
}

func TestErrDeniedSurvivesWrapping(t *testing.T) {
	denied := errors.Join(errors.New("policy engine"), ports.ErrDenied)

	if !errors.Is(denied, ports.ErrDenied) {
		t.Fatal("a denial declared by wrapping ErrDenied must be recognisable by errors.Is")
	}
	if errors.Is(errors.New("timeout dialing the policy engine"), ports.ErrDenied) {
		t.Fatal("an unrelated authorizer error must not read as a denial")
	}
}

func TestOutcomeCategoriesAreTheFourLowercaseLabels(t *testing.T) {
	want := map[ports.OutcomeCategory]string{
		ports.OutcomeAccepted: "accepted",
		ports.OutcomeRejected: "rejected",
		ports.OutcomeDenied:   "denied",
		ports.OutcomeFailed:   "failed",
	}

	for category, label := range want {
		if string(category) != label {
			t.Fatalf("OutcomeCategory = %q, want %q: the label is the metric value of outcome_category", category, label)
		}
	}
}
