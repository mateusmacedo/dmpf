package ports_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func validSpec() ports.ExecutionContextSpec {
	return ports.ExecutionContextSpec{
		RequestID:     "req-000001",
		CorrelationID: "cor-000001",
		TraceContext:  "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		Deadline:      ports.Instant(1_755_432_000_000_000_000),
		Locale:        "pt-BR",
	}
}

func subjectOf(s string) *ports.SubjectID {
	id := ports.SubjectID(s)
	return &id
}

func tenantOf(s string) *ports.TenantID {
	id := ports.TenantID(s)
	return &id
}

func causationOf(s string) *string { return &s }

func TestNewExecutionContextRejectsMissingMandatoryField(t *testing.T) {
	tests := []struct {
		name  string
		spoil func(*ports.ExecutionContextSpec)
	}{
		{name: "request_id", spoil: func(s *ports.ExecutionContextSpec) { s.RequestID = "" }},
		{name: "correlation_id", spoil: func(s *ports.ExecutionContextSpec) { s.CorrelationID = "" }},
		{name: "trace_context", spoil: func(s *ports.ExecutionContextSpec) { s.TraceContext = "" }},
		{name: "deadline", spoil: func(s *ports.ExecutionContextSpec) { s.Deadline = 0 }},
		{name: "deadline before the epoch", spoil: func(s *ports.ExecutionContextSpec) { s.Deadline = -1 }},
		{name: "locale", spoil: func(s *ports.ExecutionContextSpec) { s.Locale = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			tt.spoil(&spec)

			_, err := ports.NewExecutionContext(spec)
			if !errors.Is(err, ports.ErrContextFieldMissing) {
				t.Fatalf("omitting %s must be a construction defect, got err = %v", tt.name, err)
			}
		})
	}
}

func TestNewExecutionContextAcceptsMandatorySetAlone(t *testing.T) {
	ec, err := ports.NewExecutionContext(validSpec())
	if err != nil {
		t.Fatalf("the five mandatory fields must suffice, got err = %v", err)
	}

	if _, ok := ec.CausationID(); ok {
		t.Fatal("causation_id must report absence when the spec omits it")
	}
	if _, ok := ec.Subject(); ok {
		t.Fatal("authenticated_subject must report absence when the spec omits it")
	}
	if _, ok := ec.Tenant(); ok {
		t.Fatal("tenant_id must report absence when the spec omits it")
	}
	if got := ec.Permissions(); got != nil {
		t.Fatalf("permissions must stay absent without a subject, got %v", got)
	}
}

func TestNewExecutionContextRejectsPresentButEmptyConditional(t *testing.T) {
	tests := []struct {
		name  string
		spoil func(*ports.ExecutionContextSpec)
	}{
		{name: "causation_id", spoil: func(s *ports.ExecutionContextSpec) { s.CausationID = causationOf("") }},
		{
			name: "authenticated_subject",
			spoil: func(s *ports.ExecutionContextSpec) {
				s.Subject = subjectOf("")
				s.Permissions = []ports.Permission{}
			},
		},
		{name: "tenant_id", spoil: func(s *ports.ExecutionContextSpec) { s.Tenant = tenantOf("") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			tt.spoil(&spec)

			_, err := ports.NewExecutionContext(spec)
			if !errors.Is(err, ports.ErrContextValueEmpty) {
				t.Fatalf("a present %s must carry a resolved value, got err = %v", tt.name, err)
			}
		})
	}
}

func TestNewExecutionContextTiesPermissionsToSubject(t *testing.T) {
	tests := []struct {
		name  string
		spoil func(*ports.ExecutionContextSpec)
	}{
		{
			name:  "subject without permissions",
			spoil: func(s *ports.ExecutionContextSpec) { s.Subject = subjectOf("sub-000001") },
		},
		{
			name:  "permissions without subject",
			spoil: func(s *ports.ExecutionContextSpec) { s.Permissions = []ports.Permission{"orders:write"} },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			tt.spoil(&spec)

			_, err := ports.NewExecutionContext(spec)
			if !errors.Is(err, ports.ErrContextPermissionsMismatch) {
				t.Fatalf("%s must be a construction defect, got err = %v", tt.name, err)
			}
		})
	}
}

func TestNewExecutionContextAcceptsSubjectWithoutAnyPermission(t *testing.T) {
	spec := validSpec()
	spec.Subject = subjectOf("sub-000001")
	spec.Permissions = []ports.Permission{}

	ec, err := ports.NewExecutionContext(spec)
	if err != nil {
		t.Fatalf("a subject resolved with no effective permission is legitimate, got err = %v", err)
	}
	if got := ec.Permissions(); got == nil || len(got) != 0 {
		t.Fatalf("the empty set must stay present and distinct from absence, got %v", got)
	}
}

func TestExecutionContextCarriesEveryField(t *testing.T) {
	spec := validSpec()
	spec.CausationID = causationOf("cau-000001")
	spec.Subject = subjectOf("sub-000001")
	spec.Tenant = tenantOf("acme")
	spec.Permissions = []ports.Permission{"orders:write", "orders:read"}

	ec, err := ports.NewExecutionContext(spec)
	if err != nil {
		t.Fatalf("NewExecutionContext() err = %v", err)
	}

	if got := ec.RequestID(); got != spec.RequestID {
		t.Fatalf("RequestID() = %q, want %q", got, spec.RequestID)
	}
	if got := ec.CorrelationID(); got != spec.CorrelationID {
		t.Fatalf("CorrelationID() = %q, want %q", got, spec.CorrelationID)
	}
	if got, ok := ec.CausationID(); !ok || got != *spec.CausationID {
		t.Fatalf("CausationID() = %q, %v, want %q, true", got, ok, *spec.CausationID)
	}
	if got := ec.TraceContext(); got != spec.TraceContext {
		t.Fatalf("TraceContext() = %q, want %q", got, spec.TraceContext)
	}
	if got, ok := ec.Subject(); !ok || got != *spec.Subject {
		t.Fatalf("Subject() = %q, %v, want %q, true", got, ok, *spec.Subject)
	}
	if got, ok := ec.Tenant(); !ok || got != *spec.Tenant {
		t.Fatalf("Tenant() = %q, %v, want %q, true", got, ok, *spec.Tenant)
	}
	if got := ec.Deadline(); got != spec.Deadline {
		t.Fatalf("Deadline() = %d, want %d", got, spec.Deadline)
	}
	if got := ec.Locale(); got != spec.Locale {
		t.Fatalf("Locale() = %q, want %q", got, spec.Locale)
	}
	if got := ec.Permissions(); !slices.Equal(got, spec.Permissions) {
		t.Fatalf("Permissions() = %v, want %v", got, spec.Permissions)
	}
}

func TestExecutionContextIsImmutableAfterConstruction(t *testing.T) {
	spec := validSpec()
	spec.Subject = subjectOf("sub-000001")
	spec.Tenant = tenantOf("acme")
	spec.Permissions = []ports.Permission{"orders:write"}

	ec, err := ports.NewExecutionContext(spec)
	if err != nil {
		t.Fatalf("NewExecutionContext() err = %v", err)
	}

	spec.Permissions[0] = "orders:forged"
	*spec.Tenant = "other"
	*spec.Subject = "sub-forged"

	if got := ec.Permissions(); !slices.Equal(got, []ports.Permission{"orders:write"}) {
		t.Fatalf("mutating the spec slice must not reach the context, got %v", got)
	}
	if got, _ := ec.Tenant(); got != "acme" {
		t.Fatalf("mutating the spec tenant must not reach the context, got %q", got)
	}
	if got, _ := ec.Subject(); got != "sub-000001" {
		t.Fatalf("mutating the spec subject must not reach the context, got %q", got)
	}

	returned := ec.Permissions()
	returned[0] = "orders:forged"
	if got := ec.Permissions(); !slices.Equal(got, []ports.Permission{"orders:write"}) {
		t.Fatalf("mutating a returned slice must not reach the context, got %v", got)
	}
}

func TestExecutionContextZeroValueIsNotUsable(t *testing.T) {
	var zero ports.ExecutionContext

	if zero.RequestID() != "" || zero.Deadline() != 0 {
		t.Fatal("the zero ExecutionContext must carry no resolved value")
	}
	if _, ok := zero.Subject(); ok {
		t.Fatal("the zero ExecutionContext must report the subject as absent")
	}
	if _, ok := zero.Tenant(); ok {
		t.Fatal("the zero ExecutionContext must report the tenant as absent")
	}
}
