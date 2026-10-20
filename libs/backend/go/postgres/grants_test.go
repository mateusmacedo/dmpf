package postgres_test

import (
	"reflect"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func TestMutableGrantsReadInsertAndUpdate(t *testing.T) {
	got := postgres.Mutable("orders")

	want := postgres.Grant{Table: "orders", Privileges: []postgres.Privilege{postgres.Select, postgres.Insert, postgres.Update}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Mutable(orders) = %v, want %v", got, want)
	}
}

func TestAppendOnlyGrantsReadAndInsert(t *testing.T) {
	got := postgres.AppendOnly("ledger")

	want := postgres.Grant{Table: "ledger", Privileges: []postgres.Privilege{postgres.Select, postgres.Insert}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AppendOnly(ledger) = %v, want %v", got, want)
	}
}

func TestKernelGrantsFollowTheCapabilities(t *testing.T) {
	all := []postgres.Privilege{postgres.Select, postgres.Insert, postgres.Update, postgres.Delete}
	outbox := postgres.Grant{Table: "outbox", Privileges: all}
	inbox := postgres.Grant{Table: "inbox", Privileges: all}
	quarantine := postgres.AppendOnly("quarantine")
	cases := []struct {
		name string
		caps []postgres.Capability
		want []postgres.Grant
	}{
		{"outbox", []postgres.Capability{postgres.Outbox}, []postgres.Grant{outbox}},
		{"inbox", []postgres.Capability{postgres.Inbox}, []postgres.Grant{inbox, quarantine}},
		{"both", []postgres.Capability{postgres.Outbox, postgres.Inbox}, []postgres.Grant{outbox, inbox, quarantine}},
		{"external attempt", []postgres.Capability{postgres.ExternalAttempt}, []postgres.Grant{{Table: "external_attempt", Privileges: all}}},
		{"repeated", []postgres.Capability{postgres.Outbox, postgres.Outbox}, []postgres.Grant{outbox}},
		{"unknown", []postgres.Capability{"ledger"}, nil},
		{"none", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := postgres.KernelGrants(tc.caps...)

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("KernelGrants(%v) = %v, want %v", tc.caps, got, tc.want)
			}
		})
	}
}

func TestKernelGrantsCoverEveryKernelTable(t *testing.T) {
	caps := []postgres.Capability{postgres.Outbox, postgres.Inbox, postgres.ExternalAttempt}
	granted := map[string]bool{}
	for _, g := range postgres.KernelGrants(caps...) {
		granted[g.Table] = true
	}

	for _, table := range postgres.Tables(caps...) {
		if !granted[table] {
			t.Errorf("kernel table %s has no grant", table)
		}
	}
}

func TestKernelGrantsNeverConcedeTruncateOrReferences(t *testing.T) {
	allowed := map[postgres.Privilege]bool{postgres.Select: true, postgres.Insert: true, postgres.Update: true, postgres.Delete: true}

	for _, g := range postgres.KernelGrants(postgres.Outbox, postgres.Inbox, postgres.ExternalAttempt) {
		for _, p := range g.Privileges {
			if !allowed[p] {
				t.Errorf("grant on %s concedes %s", g.Table, p)
			}
		}
	}
}

func TestKernelGrantsAreIndependentCopies(t *testing.T) {
	first := postgres.KernelGrants(postgres.Outbox)
	first[0].Privileges[0] = "TRUNCATE"

	second := postgres.KernelGrants(postgres.Outbox)
	if second[0].Privileges[0] != postgres.Select {
		t.Fatalf("KernelGrants() shares state between calls: %v", second)
	}
}
