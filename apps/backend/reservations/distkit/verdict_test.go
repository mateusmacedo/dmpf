package distkit_test

import (
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/distkit"
)

func TestDecideAcceptsTheSingleDeliveryEffect(t *testing.T) {
	v := distkit.Decide(distkit.Effects{Inbox: 2, Reservations: 1, Outbox: 1, Items: 2, Version: 1}, distkit.Default)
	if !v.OK() {
		t.Fatalf("a single effect under redelivery reproved: %v", v.Failures())
	}
}

func TestDecideNamesTheRedeliveredIdentity(t *testing.T) {
	v := distkit.Decide(distkit.Effects{Reservations: 1, Items: 6, Version: 3}, distkit.Default)
	if v.OK() || v.Diagnostics[0].Code != distkit.CodeR004 {
		t.Fatalf("verdict = %+v, want DMPF-R004", v)
	}
	d := v.Diagnostics[0].Detail
	if !strings.Contains(d, "evt-1") || !strings.Contains(d, "evt-9") || !strings.Contains(d, "3 time(s)") {
		t.Fatalf("diagnostic does not name the redeliveries and the count: %s", d)
	}
}
