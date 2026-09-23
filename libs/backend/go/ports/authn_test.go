package ports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestCredentialPresentedAnswersTheFirstConditionAlone(t *testing.T) {
	tests := []struct {
		name string
		in   ports.Credential
		want bool
	}{
		{name: "nothing arrived", in: ports.Credential{}, want: false},
		{name: "scheme without material", in: ports.Credential{Scheme: "Bearer"}, want: false},
		{name: "material arrived", in: ports.Credential{Scheme: "Bearer", Value: "t-000001"}, want: true},
		{name: "scheme is not required to carry material", in: ports.Credential{Value: "t-000001"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Presented(); got != tt.want {
				t.Fatalf("Presented() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthenticationFailuresAreDistinctFromEachOtherAndFromDenial(t *testing.T) {
	failures := []error{
		ports.ErrCredentialAbsent,
		ports.ErrCredentialRejected,
		ports.ErrSubjectUnresolved,
	}
	for i, outer := range failures {
		for j, inner := range failures {
			if i != j && errors.Is(outer, inner) {
				t.Fatalf("the three conditions of IDN-01 must fail distinctly: %v matched %v", outer, inner)
			}
		}
		if errors.Is(outer, ports.ErrDenied) {
			t.Fatalf("authentication failure %v must not match authorization denial (IDN-06)", outer)
		}
	}
}

type stubAuthenticator struct {
	identity ports.Identity
	err      error
}

func (s stubAuthenticator) Authenticate(context.Context, ports.Credential) (ports.Identity, error) {
	return s.identity, s.err
}

func TestAuthenticatorContractResolvesIdentityOrFails(t *testing.T) {
	tenant := ports.TenantID("acme")
	resolved := ports.Identity{
		Subject:     "sub-000001",
		Tenant:      &tenant,
		Permissions: []ports.Permission{"orders:write"},
	}

	var authenticator ports.Authenticator = stubAuthenticator{identity: resolved}
	got, err := authenticator.Authenticate(t.Context(), ports.Credential{Scheme: "Bearer", Value: "t-000001"})
	if err != nil {
		t.Fatalf("Authenticate() err = %v", err)
	}
	if got.Subject != resolved.Subject {
		t.Fatalf("Subject = %q, want %q", got.Subject, resolved.Subject)
	}
	if got.Tenant == nil || *got.Tenant != tenant {
		t.Fatalf("Tenant = %v, want %q", got.Tenant, tenant)
	}

	authenticator = stubAuthenticator{err: ports.ErrCredentialRejected}
	if _, err := authenticator.Authenticate(t.Context(), ports.Credential{Value: "t-forged"}); !errors.Is(err, ports.ErrCredentialRejected) {
		t.Fatalf("Authenticate() err = %v, want %v", err, ports.ErrCredentialRejected)
	}
}

func TestIdentityCarriesAbsentTenantAsNil(t *testing.T) {
	platform := ports.Identity{Subject: "workload-000001", Permissions: []ports.Permission{}}

	if platform.Tenant != nil {
		t.Fatal("an absent tenant must stay nil, never a synthetic value (IDN-20)")
	}
}
