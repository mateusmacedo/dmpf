package domainkit_test

import (
	"strings"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/domainkit"
)

const at = orders.Instant(1755432000)

func openOrder(items, limit int) *orders.Order {
	o := orders.NewOrder("o-1", limit)
	for i := range items {
		if _, rej := o.AddItem(orders.AddItem{SKU: orders.SKU(string(rune('A' + i))), Quantity: 1, At: at}); rej != nil {
			panic(rej)
		}
	}
	return o
}

func TestRunProjectsTheAcceptingBranch(t *testing.T) {
	p := domainkit.Run(openOrder(1, 3), addItemSubject(orders.AddItem{SKU: "B", Quantity: 2, At: at}))
	if p.Branch != domainkit.Accepted {
		t.Fatalf("branch = %s", p.Branch)
	}
	if p.Response["items"] != "2" || p.Response["order"] != "o-1" {
		t.Fatalf("response = %v", p.Response)
	}
	if len(p.Events) != 1 || p.Events[0].Name != "orders.item-added" || p.Events[0].Fields["sku"] != "B" {
		t.Fatalf("events = %+v", p.Events)
	}
	if p.StateBefore["items.count"] != "1" || p.StateAfter["items.count"] != "2" {
		t.Fatalf("state before/after = %v / %v", p.StateBefore, p.StateAfter)
	}
	if len(p.Violations) != 0 {
		t.Fatalf("violations on a conforming aggregate: %v", p.Violations)
	}
}

func TestRunProjectsTheRejectingBranchWithStateUntouched(t *testing.T) {
	p := domainkit.Run(openOrder(1, 1), addItemSubject(orders.AddItem{SKU: "B", Quantity: 1, At: at}))
	if p.Branch != domainkit.Rejected || p.Rejection.Code != "orders/item-limit-exceeded" {
		t.Fatalf("projection = %+v", p)
	}
	if p.Rejection.Details["limit"] != "1" || p.Rejection.Details["attempted"] != "2" {
		t.Fatalf("details = %v", p.Rejection.Details)
	}
	if len(p.Events) != 0 {
		t.Fatalf("events under Rejected: %+v", p.Events)
	}
	v := domainkit.Equal(p, domainkit.Projection{Branch: domainkit.Rejected, Rejection: domainkit.Rejection{Code: "orders/item-limit-exceeded"}})
	if !v.OK() {
		t.Fatalf("a conforming rejection reproved: %v", v.Failures())
	}
}

func TestReadTwiceIsObservationallyEqual(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    domainkit.Verdict
	}{
		{"accepted", domainkit.ReadTwice(openOrder(1, 3), addItemSubject(orders.AddItem{SKU: "B", Quantity: 2, At: at}))},
		{"rejected", domainkit.ReadTwice(openOrder(0, 3), placeSubject(orders.PlaceOrder{At: at}))},
	} {
		if !tc.v.OK() {
			t.Errorf("%s: %v", tc.name, tc.v.Failures())
		}
	}
}

func TestEqualNamesTheFirstDivergentField(t *testing.T) {
	got := domainkit.Run(openOrder(1, 3), placeSubject(orders.PlaceOrder{At: at}))
	want := domainkit.Projection{
		Branch:   domainkit.Accepted,
		Response: domainkit.Fields{"order": "o-1"},
		Events:   []domainkit.Event{{Name: "orders.order-placed", Fields: domainkit.Fields{"order": "o-1", "items": "2", "at": "1755432000"}}},
	}
	v := domainkit.Equal(got, want)
	if len(v.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want exactly one", v.Failures())
	}
	d := v.Diagnostics[0]
	if d.Code != domainkit.CodeProjection || d.Field != "events[0].items" || d.Expected != "2" || d.Got != "1" {
		t.Fatalf("diagnostic = %+v", d)
	}
	if !strings.Contains(v.Failures()[0], "ORA-31") {
		t.Fatalf("failure text lacks the rule: %s", v.Failures()[0])
	}
}

func TestEqualReprovesABranchMismatchBeforeAnythingElse(t *testing.T) {
	got := domainkit.Run(openOrder(0, 3), placeSubject(orders.PlaceOrder{At: at}))
	v := domainkit.Equal(got, domainkit.Projection{Branch: domainkit.Accepted, Response: domainkit.Fields{"order": "o-1"}})
	if len(v.Diagnostics) != 1 || v.Diagnostics[0].Field != "branch" || v.Diagnostics[0].Got != "rejected" {
		t.Fatalf("diagnostics = %v", v.Failures())
	}
}
