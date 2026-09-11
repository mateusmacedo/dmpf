package dmpfdomain_test

import (
	"errors"
	"slices"
	"testing"

	dmpfdomain "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain"
)

func TestCodeValid(t *testing.T) {
	tests := []struct {
		name string
		code dmpfdomain.Code
		want bool
	}{
		{"context and reason", "orders/empty-order", true},
		{"digits and hyphens", "dmpf-kernel/x-1", true},
		{"single letters", "a/b", true},
		{"no slash", "EmptyOrder", false},
		{"missing reason", "orders/", false},
		{"missing context", "/empty", false},
		{"upper case", "orders/Empty-Order", false},
		{"double slash", "orders//x", false},
		{"leading hyphen", "orders/-x", false},
		{"trailing hyphen", "orders/x-", false},
		{"digit first", "1orders/x", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.code.Valid(); got != tt.want {
				t.Fatalf("Code(%q).Valid() = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestRejectPreservesFieldsInOrder(t *testing.T) {
	rej := dmpfdomain.Reject("orders/item-limit-exceeded", "item limit exceeded",
		dmpfdomain.Detail{Key: "limit", Value: "3"},
		dmpfdomain.Detail{Key: "attempted", Value: "4"},
	)

	if rej.Code() != "orders/item-limit-exceeded" {
		t.Fatalf("Code() = %q", rej.Code())
	}
	if rej.Message() != "item limit exceeded" {
		t.Fatalf("Message() = %q", rej.Message())
	}
	want := []dmpfdomain.Detail{{Key: "limit", Value: "3"}, {Key: "attempted", Value: "4"}}
	if got := rej.Details(); !slices.Equal(got, want) {
		t.Fatalf("Details() = %v, want %v", got, want)
	}
}

func TestDetailsWithoutDetailsIsEmptyNotNil(t *testing.T) {
	rej := dmpfdomain.Reject("orders/empty-order", "order has no items")

	got := rej.Details()
	if got == nil {
		t.Fatal("Details() = nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("Details() has %d elements, want 0", len(got))
	}
}

func TestDetailsReturnedSliceIsNotAliased(t *testing.T) {
	rej := dmpfdomain.Reject("orders/x", "x",
		dmpfdomain.Detail{Key: "a", Value: "1"},
		dmpfdomain.Detail{Key: "b", Value: "2"},
	)

	first := rej.Details()
	first[0], first[1] = first[1], first[0]
	first = append(first, dmpfdomain.Detail{Key: "c", Value: "3"})
	_ = first

	want := []dmpfdomain.Detail{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}}
	if got := rej.Details(); !slices.Equal(got, want) {
		t.Fatalf("Details() after mutating a previous result = %v, want %v", got, want)
	}
}

func TestRejectInputSliceIsNotAliased(t *testing.T) {
	src := []dmpfdomain.Detail{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}}
	rej := dmpfdomain.Reject("orders/x", "x", src...)

	src[0] = dmpfdomain.Detail{Key: "mutated", Value: "9"}

	want := []dmpfdomain.Detail{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}}
	if got := rej.Details(); !slices.Equal(got, want) {
		t.Fatalf("Details() after mutating the input slice = %v, want %v", got, want)
	}
}

func TestRejectionError(t *testing.T) {
	rej := dmpfdomain.Reject("orders/empty-order", "order has no items")

	if got := rej.Error(); got != "orders/empty-order: order has no items" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestRejectionIsRecoverableThroughErrorsAs(t *testing.T) {
	rej := dmpfdomain.Reject("orders/empty-order", "order has no items")
	wrapped := errors.Join(rej)

	var target *dmpfdomain.Rejection
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As did not recover *Rejection from the wrapped error")
	}
	if target.Code() != "orders/empty-order" {
		t.Fatalf("recovered Code() = %q", target.Code())
	}
}
