package api_test

import (
	"net/http"
	"strings"
	"testing"
)

const (
	noCredential      = ""
	forgedCredential  = "Bearer not-a-declaration"
	tenantlessSubject = `Bearer {"sub":"tester","permissions":[]}`
)

func TestARequestWithoutACredentialNeverReachesAContext(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	cases := []struct {
		name, method, path string
	}{
		{name: "addItem", method: http.MethodPost, path: "/orders/o-1/items"},
		{name: "placeOrder", method: http.MethodPost, path: "/orders/o-1/place"},
		{name: "findOrder", method: http.MethodGet, path: "/orders/o-1"},
		{name: "findReservation", method: http.MethodGet, path: "/reservations/o-1"},
		{name: "reserve", method: http.MethodPost, path: "/reservations/o-1/reserve"},
		{name: "cancel", method: http.MethodPost, path: "/reservations/o-1/cancel"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.do(t, tc.method, tc.path, nil, "Authorization", noCredential, "Idempotency-Key", "k-1")

			requireRejection(t, rec, http.StatusUnauthorized, "unauthenticated")
		})
	}

	if n := f.fake.total(); n != 0 {
		t.Fatalf("calls = %d, want 0: the edge denies before the application service", n)
	}
}

func TestAForgedCredentialIsRefusedAsUnauthenticated(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	rec := f.do(t, http.MethodGet, "/orders/o-1", nil, "Authorization", forgedCredential)

	requireRejection(t, rec, http.StatusUnauthorized, "unauthenticated")
	if n := f.fake.total(); n != 0 {
		t.Fatalf("calls = %d, want 0", n)
	}
}

// A subject that verified but carries no tenant is authenticated and still
// denied: IDN-06 keeps the two refusals apart, and answering 401 here would
// tell the caller the wrong thing about the wrong problem.
func TestTheTenantCrossesTheFanOutAndTheSubjectNeverDoes(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, http.MethodGet, "/orders/o-1", nil)

	md := f.fake.callsTo("FindOrder")[0].md
	if got := md.Get("x-tenant-id"); len(got) != 1 || got[0] != "public" {
		t.Fatalf("x-tenant-id = %v, want the resolved tenant preserved (CTX-13)", got)
	}

	// The subject of testCredential must appear under no key at all: CTX-12
	// turns it into provenance, and asserting it across the hop would let the
	// callee trust an identity it did not resolve (IDN-02).
	for key, values := range md {
		for _, value := range values {
			if strings.Contains(value, "tester") {
				t.Fatalf("metadata %q carries %q: the subject does not cross the fan-out (CTX-12)", key, value)
			}
		}
	}
}

func TestAnAuthenticatedSubjectWithoutTenantIsForbiddenNotUnauthenticated(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	rec := f.do(t, http.MethodGet, "/orders/o-1", nil, "Authorization", tenantlessSubject)

	requireRejection(t, rec, http.StatusForbidden, "tenant-unresolved")
	if n := f.fake.total(); n != 0 {
		t.Fatalf("calls = %d, want 0", n)
	}
}
