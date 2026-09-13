package metrics_test

import (
	"reflect"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
)

func measurement(l metrics.Labels) metric.MeasurementOption {
	return metric.WithAttributes(l.Attributes()...)
}

// forbidden are the substrings MET-07 keeps out of a label key (unbounded or
// personal). TenantWithin is the nominal exception of MET-07/MET-12, not a
// one-string builder, covered by TestTenantEntersOnlyThroughADeclaredAllowlist.
var forbidden = []string{"id", "message", "correlation", "request", "user", "tenant"}

// builderMethods are the methods of Labels that take one string and return
// Labels — that is, every way a key can enter a series without an allowlist.
func builderMethods() []reflect.Method {
	labels := reflect.TypeOf(metrics.Labels{})
	stringType := reflect.TypeOf("")
	found := make([]reflect.Method, 0, labels.NumMethod())

	for i := range labels.NumMethod() {
		method := labels.Method(i)
		signature := method.Type
		if signature.NumIn() == 2 && signature.In(1) == stringType &&
			signature.NumOut() == 1 && signature.Out(0) == labels {
			found = append(found, method)
		}
	}
	return found
}

func TestNoBuilderMethodProducesAForbiddenKey(t *testing.T) {
	methods := builderMethods()
	if len(methods) == 0 {
		t.Fatal("reflection found no builder method on Labels: the check would pass vacuously")
	}

	for _, method := range methods {
		out := method.Func.Call([]reflect.Value{
			reflect.ValueOf(metrics.Labels{}),
			reflect.ValueOf("value"),
		})
		produced := out[0].Interface().(metrics.Labels).Attributes()

		if len(produced) != 1 {
			t.Fatalf("%s produced %d attributes, want 1", method.Name, len(produced))
		}
		key := string(produced[0].Key)
		for _, term := range forbidden {
			if strings.Contains(key, term) {
				t.Errorf("%s produces key %q, which contains the forbidden term %q (MET-07)", method.Name, key, term)
			}
		}
	}
}

func TestThereIsNoGenericSetter(t *testing.T) {
	labels := reflect.TypeOf(metrics.Labels{})
	stringType := reflect.TypeOf("")

	for i := range labels.NumMethod() {
		method := labels.Method(i)
		signature := method.Type
		if signature.NumIn() == 3 && signature.In(1) == stringType && signature.In(2) == stringType {
			t.Fatalf("%s takes a key and a value: a generic setter would defeat the allowlist (MET-04)", method.Name)
		}
	}
}

func TestEveryPermittedKeyHasABuilderMethod(t *testing.T) {
	produced := map[string]bool{}
	for _, method := range builderMethods() {
		out := method.Func.Call([]reflect.Value{
			reflect.ValueOf(metrics.Labels{}),
			reflect.ValueOf("value"),
		})
		for _, kv := range out[0].Interface().(metrics.Labels).Attributes() {
			produced[string(kv.Key)] = true
		}
	}

	for _, key := range []string{
		metrics.KeyDependency, metrics.KeyOperation, metrics.KeyService,
		metrics.KeyErrorCategory, metrics.KeyOutcomeCategory, metrics.KeyRoute,
	} {
		if !produced[key] {
			t.Errorf("no builder method produces the permitted key %q", key)
		}
	}
}

func TestTheBuilderAccumulatesInOrder(t *testing.T) {
	got := metrics.Labels{}.
		Dependency("payments").
		Operation("Authorize").
		ErrorCategory("timeout").
		Attributes()

	want := []attribute.KeyValue{
		attribute.String(metrics.KeyDependency, "payments"),
		attribute.String(metrics.KeyOperation, "Authorize"),
		attribute.String(metrics.KeyErrorCategory, "timeout"),
	}
	if len(got) != len(want) {
		t.Fatalf("Attributes() has %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Attributes()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestAnEmptyValueIsOmitted(t *testing.T) {
	got := metrics.Labels{}.Dependency("payments").Operation("").Attributes()

	if len(got) != 1 {
		t.Fatalf("Attributes() = %v, want only the dependency: an empty label is omitted, not recorded blank", got)
	}
}

func TestABuiltSetIsSafeToShare(t *testing.T) {
	base := metrics.Labels{}.Dependency("payments")

	first := base.Operation("Authorize")
	second := base.Operation("Capture")

	if len(base.Attributes()) != 1 {
		t.Fatalf("the base set grew to %v: the builder must return a new value", base.Attributes())
	}
	if first.Attributes()[1].Value.AsString() != "Authorize" {
		t.Fatalf("the first branch reads %v, want Authorize — the branches must not share an array", first.Attributes())
	}
	if second.Attributes()[1].Value.AsString() != "Capture" {
		t.Fatalf("the second branch reads %v, want Capture", second.Attributes())
	}
}

func TestAttributesReturnsACopy(t *testing.T) {
	labels := metrics.Labels{}.Dependency("payments")

	labels.Attributes()[0] = attribute.String("tampered", "x")

	if got := string(labels.Attributes()[0].Key); got != metrics.KeyDependency {
		t.Fatalf("key = %q after rewriting the returned slice, want %q", got, metrics.KeyDependency)
	}
}

func TestTenantEntersOnlyThroughADeclaredAllowlist(t *testing.T) {
	allowlist, err := metrics.DeclareTenants("acme", "globex")
	if err != nil {
		t.Fatalf("DeclareTenants() = %v, want nil", err)
	}

	t.Run("a declared tenant is recorded as itself", func(t *testing.T) {
		got := metrics.Labels{}.TenantWithin(allowlist, "acme").Attributes()
		if len(got) != 1 || string(got[0].Key) != metrics.KeyTenant || got[0].Value.AsString() != "acme" {
			t.Fatalf("Attributes() = %v, want tenant=acme", got)
		}
	})

	t.Run("an undeclared tenant collapses into other", func(t *testing.T) {
		got := metrics.Labels{}.TenantWithin(allowlist, "initech").Attributes()
		if got[0].Value.AsString() != metrics.OtherTenant {
			t.Fatalf("Attributes() = %v, want tenant=%s (MET-07)", got, metrics.OtherTenant)
		}
	})

	t.Run("an empty tenant is other, never omitted", func(t *testing.T) {
		got := metrics.Labels{}.TenantWithin(allowlist, "").Attributes()
		if len(got) != 1 || got[0].Value.AsString() != metrics.OtherTenant {
			t.Fatalf("Attributes() = %v, want tenant=%s", got, metrics.OtherTenant)
		}
	})

	t.Run("the zero allowlist declares nobody", func(t *testing.T) {
		got := metrics.Labels{}.TenantWithin(metrics.Tenants{}, "acme").Attributes()
		if got[0].Value.AsString() != metrics.OtherTenant {
			t.Fatalf("Attributes() = %v, want tenant=%s", got, metrics.OtherTenant)
		}
	})

	t.Run("no builder takes a tenant without an allowlist", func(t *testing.T) {
		for _, method := range builderMethods() {
			out := method.Func.Call([]reflect.Value{reflect.ValueOf(metrics.Labels{}), reflect.ValueOf("v")})
			for _, kv := range out[0].Interface().(metrics.Labels).Attributes() {
				if string(kv.Key) == metrics.KeyTenant {
					t.Fatalf("%s produces the tenant key without an allowlist (MET-07)", method.Name)
				}
			}
		}
	})
}

func TestDeclareTenantsRefusesAnEmptyDeclaration(t *testing.T) {
	if _, err := metrics.DeclareTenants(); err == nil {
		t.Fatal("DeclareTenants() = nil error, want a refusal: an empty set is not a declared set")
	}
	if _, err := metrics.DeclareTenants("acme", ""); err == nil {
		t.Fatal("DeclareTenants(\"acme\", \"\") = nil error, want a refusal of the empty name")
	}
}
