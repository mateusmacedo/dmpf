package api_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/api"
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
	if got := md.Get("x-tenant-id"); len(got) != 1 || got[0] != "acme" {
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

// CTX-11, locale column: the edge preserves the declared locale and the hop
// downstream preserves it too, so the contexts answer in the caller's language.
func TestTheDeclaredLocaleCrossesTheFanOut(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, http.MethodGet, "/orders/o-1", nil, "Accept-Language", "pt-BR,en;q=0.8")

	md := f.fake.callsTo("FindOrder")[0].md
	if got := md.Get("x-locale"); len(got) != 1 || got[0] != "pt-BR" {
		t.Fatalf("x-locale = %v, want pt-BR preserved from the edge", got)
	}
}

// A value that is not a language tag never reaches the metadata: gRPC refuses
// anything outside printable ASCII, and the edge default is what CTX-01 wants.
func TestAMalformedLocaleFallsBackToTheEdgeDefault(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, http.MethodGet, "/orders/o-1", nil, "Accept-Language", "pt\x7fBR")

	md := f.fake.callsTo("FindOrder")[0].md
	if got := md.Get("x-locale"); len(got) != 1 || got[0] != api.DefaultLocale {
		t.Fatalf("x-locale = %v, want the edge default %q", got, api.DefaultLocale)
	}
}

// CTX-06 at the edge: a header asserting another tenant than the credential
// resolved is an elevation attempt, refused before any context is called.
func TestAHeaderAssertingAnotherTenantIsRefused(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	rec := f.do(t, http.MethodGet, "/orders/o-1", nil, "X-Tenant-ID", "globex")

	requireRejection(t, rec, http.StatusForbidden, "identity-mismatch")
	if n := f.fake.total(); n != 0 {
		t.Fatalf("calls = %d, want 0", n)
	}
}

func TestAHeaderRepeatingTheResolvedTenantIsRedundantNotRefused(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	if rec := f.do(t, http.MethodGet, "/orders/o-1", nil, "X-Tenant-ID", "acme"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: an equal value is redundant, and still never the source", rec.Code)
	}
}

// CTX-11 ingress and downstream for request_id and causation_id: the edge mints
// its own request id per request, ignoring any the client names, and sends it
// downstream as the causation, so the callee's cause is this edge step.
func TestTheEdgeMintsItsRequestIDAndSendsItAsTheCausation(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, http.MethodGet, "/orders/o-1", nil, "X-Request-ID", "client-chosen", "X-Causation-ID", "client-chosen")
	f.do(t, http.MethodGet, "/orders/o-1", nil)

	calls := f.fake.callsTo("FindOrder")
	first, second := calls[0].md.Get("x-causation-id"), calls[1].md.Get("x-causation-id")
	if len(first) != 1 || len(second) != 1 || first[0] == "" || first[0] == second[0] {
		t.Fatalf("x-causation-id = %v then %v, want one fresh edge request id per request", first, second)
	}
	if first[0] == "client-chosen" {
		t.Fatal("the causation downstream is what the client named; the edge must regenerate it")
	}
}

// IDN-16 at the edge: an authenticated subject of the right tenant without the
// route's permission is refused before any context is called, because CTX-12
// keeps the subject from reaching the context that would otherwise decide.
func TestASubjectWithoutTheRoutesPermissionIsRefusedAtTheEdge(t *testing.T) {
	f := newFixture(t, &fakeContexts{})
	readOnly := `Bearer {"sub":"tester","tenant":"acme","permissions":["orders:read"]}`

	rec := f.do(t, http.MethodPost, "/orders/o-1/items", strings.NewReader(`{"sku":"A","quantity":1}`),
		"Idempotency-Key", "k-1", "Content-Type", "application/json", "Authorization", readOnly)

	requireRejection(t, rec, http.StatusForbidden, "permission-denied")
	if n := f.fake.total(); n != 0 {
		t.Fatalf("calls = %d, want 0", n)
	}
}
