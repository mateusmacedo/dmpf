package metrics_test

import (
	"reflect"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
)

func measurement(l metrics.Labels) metric.MeasurementOption {
	return metric.WithAttributes(l.Attributes()...)
}

// forbidden are the substrings MET-07 keeps out of a label key: an identifier
// or a message is unbounded, and a correlation, request, user or tenant is
// personal or high cardinality.
var forbidden = []string{"id", "message", "correlation", "request", "user", "tenant"}

// builderMethods are the methods of Labels that take one string and return
// Labels — that is, every way a key can enter a series.
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
		metrics.KeyErrorCategory, metrics.KeyOutcomeCategory,
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
