package dmpfapplication_test

import (
	"testing"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
)

type response struct {
	Value string
}

func TestAcceptedCarriesTheResponseAndNoRejection(t *testing.T) {
	want := response{Value: "ok"}

	out := dmpfapplication.Accepted(want)

	if got := out.Response(); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	rej, refused := out.Rejection()
	if refused {
		t.Fatal("Rejection() reported a refusal on the accepting branch")
	}
	if rej != nil {
		t.Fatalf("Rejection() = %v, want nil", rej)
	}
}

func TestRejectedCarriesTheRejectionAndTheZeroResponse(t *testing.T) {
	want := dmpfdomain.Reject("orders/item-limit-exceeded", "item limit exceeded")

	out := dmpfapplication.Rejected[response](want)

	rej, refused := out.Rejection()
	if !refused {
		t.Fatal("Rejection() did not report a refusal on the rejecting branch")
	}
	if rej != want {
		t.Fatalf("Rejection() = %v, want the very rejection the UPR produced", rej)
	}
	if got := out.Response(); got != (response{}) {
		t.Fatalf("Response() = %+v, want the zero response", got)
	}
}

func TestRejectedPanicsOnNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Rejected(nil) must panic: absence of rejection is not a third outcome (DEC-01)")
		}
	}()

	_ = dmpfapplication.Rejected[response](nil)
}
