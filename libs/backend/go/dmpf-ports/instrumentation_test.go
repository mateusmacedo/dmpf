package dmpfports_test

import (
	"context"
	"errors"
	"testing"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

func TestNoInstrumentationKeepsTheContextAndEndsInertly(t *testing.T) {
	type key struct{}
	parent := context.WithValue(context.Background(), key{}, "marker")

	ctx, end := dmpfports.NoInstrumentation().BeginOperation(parent, "orders.AddItem")

	if ctx.Value(key{}) != "marker" {
		t.Fatal("BeginOperation dropped the parent context; the inert hook must not alter propagation")
	}
	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})
}

func TestNoInstrumentationAuditIsInert(t *testing.T) {
	dmpfports.NoInstrumentation().Audit(context.Background(), dmpfports.AuditEvent{
		Object:  "order-1",
		Action:  "orders.AddItem",
		Outcome: dmpfports.OutcomeAccepted,
		At:      dmpfports.Instant(1),
	})
}

func TestResultCarriesTheRawErrorForTheProviderToClassify(t *testing.T) {
	cause := errors.New("connection reset by peer")

	result := dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: cause}

	if !errors.Is(result.Err, cause) {
		t.Fatal("Result.Err must carry the raw error: the taxonomy of MET-10 belongs to the provider")
	}
}

func TestResultOnAcceptedCarriesNoError(t *testing.T) {
	result := dmpfports.Result{Outcome: dmpfports.OutcomeAccepted}

	if result.Err != nil {
		t.Fatalf("Result.Err = %v on the accepting branch, want nil", result.Err)
	}
}

func TestErrDeniedSurvivesWrapping(t *testing.T) {
	denied := errors.Join(errors.New("policy engine"), dmpfports.ErrDenied)

	if !errors.Is(denied, dmpfports.ErrDenied) {
		t.Fatal("a denial declared by wrapping ErrDenied must be recognisable by errors.Is")
	}
	if errors.Is(errors.New("timeout dialing the policy engine"), dmpfports.ErrDenied) {
		t.Fatal("an unrelated authorizer error must not read as a denial")
	}
}

func TestOutcomeCategoriesAreTheFourLowercaseLabels(t *testing.T) {
	want := map[dmpfports.OutcomeCategory]string{
		dmpfports.OutcomeAccepted: "accepted",
		dmpfports.OutcomeRejected: "rejected",
		dmpfports.OutcomeDenied:   "denied",
		dmpfports.OutcomeFailed:   "failed",
	}

	for category, label := range want {
		if string(category) != label {
			t.Fatalf("OutcomeCategory = %q, want %q: the label is the metric value of outcome_category", category, label)
		}
	}
}
