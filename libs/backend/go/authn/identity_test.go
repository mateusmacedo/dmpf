package authn_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func request(t *testing.T, header string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/orders/o-1", nil)
	if header != "" {
		r.Header.Set("Authorization", header)
	}
	return r
}

func TestCredentialFromReadsTheAuthorizationHeader(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantScheme string
		wantValue  string
	}{
		{name: "bearer", header: "Bearer t-000001", wantScheme: "Bearer", wantValue: "t-000001"},
		{name: "lowercase scheme", header: "bearer t-000001", wantScheme: "bearer", wantValue: "t-000001"},
		{name: "extra spacing", header: "Bearer    t-000001", wantScheme: "Bearer", wantValue: "t-000001"},
		{name: "no header", header: ""},
		{name: "scheme alone", header: "Bearer", wantScheme: "Bearer"},
		{name: "material without scheme", header: "t-000001", wantScheme: "t-000001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authn.CredentialFrom(request(t, tt.header))

			if got.Scheme != tt.wantScheme {
				t.Fatalf("Scheme = %q, want %q", got.Scheme, tt.wantScheme)
			}
			if got.Value != tt.wantValue {
				t.Fatalf("Value = %q, want %q", got.Value, tt.wantValue)
			}
		})
	}
}

func TestCredentialFromNeverLeaksTheHeaderIntoTheValueWhenAbsent(t *testing.T) {
	if got := authn.CredentialFrom(request(t, "")); got.Presented() {
		t.Fatalf("Presented() = true for a request carrying no Authorization header, got %+v", got)
	}
}

func TestDevAuthenticatorResolvesTheIdentityTheCredentialDeclares(t *testing.T) {
	var authenticator ports.Authenticator = authn.DevAuthenticator{}

	got, err := authenticator.Authenticate(t.Context(), ports.Credential{
		Scheme: "Bearer",
		Value:  `{"sub":"alice","tenant":"acme","permissions":["orders:write","orders:read"]}`,
	})
	if err != nil {
		t.Fatalf("Authenticate() err = %v", err)
	}

	if got.Subject != "alice" {
		t.Fatalf("Subject = %q, want %q", got.Subject, "alice")
	}
	if got.Tenant == nil || *got.Tenant != "acme" {
		t.Fatalf("Tenant = %v, want %q", got.Tenant, "acme")
	}
	if want := []ports.Permission{"orders:read", "orders:write"}; !slices.Equal(got.Permissions, want) {
		t.Fatalf("Permissions = %v, want %v", got.Permissions, want)
	}
}

func TestDevAuthenticatorLeavesAnUndeclaredTenantAbsent(t *testing.T) {
	got, err := authn.DevAuthenticator{}.Authenticate(t.Context(), ports.Credential{
		Scheme: "Bearer",
		Value:  `{"sub":"workload"}`,
	})
	if err != nil {
		t.Fatalf("Authenticate() err = %v", err)
	}

	if got.Tenant != nil {
		t.Fatalf("Tenant = %v, want nil: absence keeps its own shape (IDN-20)", *got.Tenant)
	}
	if got.Permissions == nil {
		t.Fatal("a resolved subject always carries a set, empty at worst")
	}
}

func TestDevAuthenticatorFailsTheThreeConditionsDistinctly(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  error
	}{
		{name: "nothing presented", value: "", want: ports.ErrCredentialAbsent},
		{name: "not a declaration", value: "not-json", want: ports.ErrCredentialRejected},
		{name: "declaration without a subject", value: `{"tenant":"acme"}`, want: ports.ErrSubjectUnresolved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := authn.DevAuthenticator{}.Authenticate(t.Context(), ports.Credential{Scheme: "Bearer", Value: tt.value})

			if !errors.Is(err, tt.want) {
				t.Fatalf("Authenticate() err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestDevAuthenticatorRefusesAnUnsupportedScheme(t *testing.T) {
	_, err := authn.DevAuthenticator{}.Authenticate(t.Context(), ports.Credential{
		Scheme: "Basic",
		Value:  `{"sub":"alice"}`,
	})

	if !errors.Is(err, ports.ErrCredentialRejected) {
		t.Fatalf("Authenticate() err = %v, want %v", err, ports.ErrCredentialRejected)
	}
}
