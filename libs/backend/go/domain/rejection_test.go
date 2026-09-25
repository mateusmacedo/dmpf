package domain_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func TestCodeValid(t *testing.T) {
	tests := []struct {
		name string
		code domain.Code
		want bool
	}{
		{"context and reason", "orders/empty-order", true},
		{"digits and hyphens", "kernel/x-1", true},
		{"single letters", "a/b", true},
		{"context, aggregate and reason", "resource-scheduling/booking/not-reserved", true},
		{"four segments", "a/b/c/d", false},
		{"missing aggregate", "orders//empty-order", false},
		{"trailing slash after aggregate", "orders/order/", false},
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
	rej := domain.Reject("orders/item-limit-exceeded", "item limit exceeded",
		domain.Detail{Key: "limit", Value: "3"},
		domain.Detail{Key: "attempted", Value: "4"},
	)

	if rej.Code() != "orders/item-limit-exceeded" {
		t.Fatalf("Code() = %q", rej.Code())
	}
	if rej.Message() != "item limit exceeded" {
		t.Fatalf("Message() = %q", rej.Message())
	}
	want := []domain.Detail{{Key: "limit", Value: "3"}, {Key: "attempted", Value: "4"}}
	if got := rej.Details(); !slices.Equal(got, want) {
		t.Fatalf("Details() = %v, want %v", got, want)
	}
}

func TestDetailsWithoutDetailsIsEmptyNotNil(t *testing.T) {
	rej := domain.Reject("orders/empty-order", "order has no items")

	got := rej.Details()
	if got == nil {
		t.Fatal("Details() = nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("Details() has %d elements, want 0", len(got))
	}
}

func TestDetailsReturnedSliceIsNotAliased(t *testing.T) {
	rej := domain.Reject("orders/x", "x",
		domain.Detail{Key: "a", Value: "1"},
		domain.Detail{Key: "b", Value: "2"},
	)

	first := rej.Details()
	first[0], first[1] = first[1], first[0]
	first = append(first, domain.Detail{Key: "c", Value: "3"})
	_ = first

	want := []domain.Detail{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}}
	if got := rej.Details(); !slices.Equal(got, want) {
		t.Fatalf("Details() after mutating a previous result = %v, want %v", got, want)
	}
}

func TestRejectInputSliceIsNotAliased(t *testing.T) {
	src := []domain.Detail{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}}
	rej := domain.Reject("orders/x", "x", src...)

	src[0] = domain.Detail{Key: "mutated", Value: "9"}

	want := []domain.Detail{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}}
	if got := rej.Details(); !slices.Equal(got, want) {
		t.Fatalf("Details() after mutating the input slice = %v, want %v", got, want)
	}
}

func TestRejectionError(t *testing.T) {
	rej := domain.Reject("orders/empty-order", "order has no items")

	if got := rej.Error(); got != "orders/empty-order: order has no items" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestRejectionIsRecoverableThroughErrorsAs(t *testing.T) {
	rej := domain.Reject("orders/empty-order", "order has no items")
	wrapped := errors.Join(rej)

	var target *domain.Rejection
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As did not recover *Rejection from the wrapped error")
	}
	if target.Code() != "orders/empty-order" {
		t.Fatalf("recovered Code() = %q", target.Code())
	}
}
