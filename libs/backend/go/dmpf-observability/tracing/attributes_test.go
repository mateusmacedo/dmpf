package tracing_test

import (
	"reflect"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
)

func TestTenantIDIsOmittedWhenAbsent(t *testing.T) {
	got := tracing.Attributes{}.Service("orders").TenantID("").KeyValues()

	if len(got) != 1 {
		t.Fatalf("KeyValues() = %v, want only the service: absence is information (CTX-26)", got)
	}
}

func TestEveryStringBuilderOmitsAnEmptyValue(t *testing.T) {
	stringMethods := builderMethods(reflect.TypeOf(""))
	if len(stringMethods) == 0 {
		t.Fatal("reflection found no string builder on Attributes")
	}

	for _, method := range stringMethods {
		out := method.Func.Call([]reflect.Value{
			reflect.ValueOf(tracing.Attributes{}),
			reflect.ValueOf(""),
		})
		if got := out[0].Interface().(tracing.Attributes).KeyValues(); len(got) != 0 {
			t.Errorf("%s(\"\") produced %v, want nothing", method.Name, got)
		}
	}
}

func TestAttemptZeroIsRecorded(t *testing.T) {
	got := tracing.Attributes{}.Attempt(0).KeyValues()

	if len(got) != 1 || got[0].Value.AsInt64() != 0 {
		t.Fatalf("KeyValues() = %v, want the attempt recorded as 0: it distinguishes the first call from no instrumentation", got)
	}
}

// builderMethods are the methods of Attributes taking one value of the given
// type and returning Attributes — that is, every way an attribute can be set.
func builderMethods(param reflect.Type) []reflect.Method {
	attributes := reflect.TypeOf(tracing.Attributes{})
	found := make([]reflect.Method, 0, attributes.NumMethod())

	for i := range attributes.NumMethod() {
		method := attributes.Method(i)
		signature := method.Type
		if signature.NumIn() == 2 && signature.In(1) == param &&
			signature.NumOut() == 1 && signature.Out(0) == attributes {
			found = append(found, method)
		}
	}
	return found
}

func TestNoBuilderMethodAcceptsAnErrorOrAnInterface(t *testing.T) {
	attributes := reflect.TypeOf(tracing.Attributes{})
	errorType := reflect.TypeOf((*error)(nil)).Elem()

	for i := range attributes.NumMethod() {
		method := attributes.Method(i)
		signature := method.Type
		for argument := 1; argument < signature.NumIn(); argument++ {
			in := signature.In(argument)
			if in == errorType {
				t.Errorf("%s accepts an error: the message would reach the span (TRC-15)", method.Name)
			}
			if in.Kind() == reflect.Interface {
				t.Errorf("%s accepts %v, an interface: a payload or a domain type could enter through it", method.Name, in)
			}
		}
	}
}

func TestThereIsNoGenericSetter(t *testing.T) {
	attributes := reflect.TypeOf(tracing.Attributes{})
	stringType := reflect.TypeOf("")

	for i := range attributes.NumMethod() {
		method := attributes.Method(i)
		if signature := method.Type; signature.NumIn() == 3 &&
			signature.In(1) == stringType && signature.In(2) == stringType {
			t.Fatalf("%s takes a key and a value: a generic setter would defeat the allowlist (TRC-04)", method.Name)
		}
	}
}

func TestEveryKeyCarriesThePlatformPrefix(t *testing.T) {
	for _, method := range builderMethods(reflect.TypeOf("")) {
		out := method.Func.Call([]reflect.Value{
			reflect.ValueOf(tracing.Attributes{}),
			reflect.ValueOf("value"),
		})
		for _, kv := range out[0].Interface().(tracing.Attributes).KeyValues() {
			if !strings.HasPrefix(string(kv.Key), "dmpf.") {
				t.Errorf("%s produces key %q, which lacks the dmpf. prefix", method.Name, kv.Key)
			}
		}
	}
}

func TestTheBuilderAccumulatesInOrder(t *testing.T) {
	got := tracing.Attributes{}.
		Service("orders").
		Operation("AddItem").
		TrafficClass("write").
		KeyValues()

	want := []attribute.KeyValue{
		attribute.String(tracing.KeyService, "orders"),
		attribute.String(tracing.KeyOperation, "AddItem"),
		attribute.String(tracing.KeyTrafficClass, "write"),
	}
	if len(got) != len(want) {
		t.Fatalf("KeyValues() has %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("KeyValues()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestABuiltSetIsSafeToShare(t *testing.T) {
	base := tracing.Attributes{}.Service("orders")

	first := base.Operation("AddItem")
	second := base.Operation("PlaceOrder")

	if len(base.KeyValues()) != 1 {
		t.Fatalf("the base set grew to %v: the builder must return a new value", base.KeyValues())
	}
	if first.KeyValues()[1].Value.AsString() != "AddItem" {
		t.Fatalf("the first branch reads %v, want AddItem", first.KeyValues())
	}
	if second.KeyValues()[1].Value.AsString() != "PlaceOrder" {
		t.Fatalf("the second branch reads %v, want PlaceOrder", second.KeyValues())
	}
}

func TestKeyValuesReturnsACopy(t *testing.T) {
	attributes := tracing.Attributes{}.Service("orders")

	attributes.KeyValues()[0] = attribute.String("tampered", "x")

	if got := string(attributes.KeyValues()[0].Key); got != tracing.KeyService {
		t.Fatalf("key = %q after rewriting the returned slice, want %q", got, tracing.KeyService)
	}
}
