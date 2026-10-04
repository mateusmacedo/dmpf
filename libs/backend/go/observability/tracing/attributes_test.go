package tracing_test

import (
	"reflect"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func TestTenantIDIsOmittedWhenAbsent(t *testing.T) {
	got := tracing.Attributes{}.RequestID("r-1").TenantID("").KeyValues()

	if len(got) != 1 {
		t.Fatalf("KeyValues() = %v, want only the request: absence is information (CTX-26)", got)
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
		t.Fatalf("KeyValues() = %v, want the attempt recorded as 0: a number is never omitted the way an empty string is", got)
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

var semconvVocabulary = map[attribute.Key]bool{
	semconv.ErrorTypeKey:                       true,
	semconv.HTTPRequestMethodKey:               true,
	semconv.HTTPRouteKey:                       true,
	semconv.HTTPResponseStatusCodeKey:          true,
	semconv.RPCSystemNameKey:                   true,
	semconv.RPCMethodKey:                       true,
	semconv.RPCResponseStatusCodeKey:           true,
	semconv.MessagingSystemKey:                 true,
	semconv.MessagingOperationNameKey:          true,
	semconv.MessagingOperationTypeKey:          true,
	semconv.MessagingDestinationNameKey:        true,
	semconv.MessagingDestinationPartitionIDKey: true,
	semconv.MessagingConsumerGroupNameKey:      true,
	semconv.MessagingMessageIDKey:              true,
	semconv.MessagingMessageConversationIDKey:  true,
	semconv.MessagingBatchMessageCountKey:      true,
	semconv.MessagingKafkaOffsetKey:            true,
	semconv.CloudEventsEventIDKey:              true,
	semconv.CloudEventsEventSourceKey:          true,
	semconv.CloudEventsEventTypeKey:            true,
	semconv.DBCollectionNameKey:                true,
	semconv.DBOperationNameKey:                 true,
}

func canonicalKey(key string) bool {
	return strings.HasPrefix(key, "dmpf.") || semconvVocabulary[attribute.Key(key)]
}

func TestEveryKeyIsAPlatformKeyOrASemconvConstant(t *testing.T) {
	for method, keys := range builtKeys(t) {
		for _, key := range keys {
			if !canonicalKey(key) {
				t.Errorf("%s produces key %q, neither dmpf.* nor a semconv v1.43.0 constant (RF-B1)", method, key)
			}
		}
	}
}

func TestTheCanonicalKeyPredicate(t *testing.T) {
	for key, want := range map[string]bool{
		string(semconv.MessagingKafkaOffsetKey):            true,
		string(semconv.MessagingDestinationPartitionIDKey): true,
		tracing.KeyCorrelationID:                           true,
		"payload.body":                                     false,
		"partition":                                        false,
		"messaging.kafka.partition":                        false,
		"dmpfx.leak":                                       false,
		"":                                                 false,
	} {
		if got := canonicalKey(key); got != want {
			t.Errorf("canonicalKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestTheSemconvBuildersKeepTheSemconvTypes(t *testing.T) {
	attributes := reflect.TypeOf(tracing.Attributes{})
	for name, param := range map[string]reflect.Type{
		"MessagingDestinationPartitionID": reflect.TypeOf(""),
		"MessagingKafkaOffset":            reflect.TypeOf(0),
	} {
		method, found := attributes.MethodByName(name)
		if !found {
			t.Errorf("Attributes has no %s builder", name)
			continue
		}
		if got := method.Type.In(1); got != param {
			t.Errorf("%s takes %v, want %v", name, got, param)
		}
	}

	got := tracing.Attributes{}.MessagingDestinationPartitionID("3").MessagingKafkaOffset(0).KeyValues()
	want := []attribute.KeyValue{semconv.MessagingDestinationPartitionID("3"), semconv.MessagingKafkaOffset(0)}
	if len(got) != len(want) {
		t.Fatalf("KeyValues() = %v, want %v: offset 0 is a real offset", got, want)
	}
	for i := range want {
		if got[i] != want[i] || got[i].Value.Type() != want[i].Value.Type() {
			t.Errorf("KeyValues()[%d] = %v (%v), want %v (%v)", i, got[i], got[i].Value.Type(), want[i], want[i].Value.Type())
		}
	}
}

func vocabulary() map[string]string {
	return map[string]string{
		"KeyCorrelationID":         tracing.KeyCorrelationID,
		"KeyRequestID":             tracing.KeyRequestID,
		"KeyTenantID":              tracing.KeyTenantID,
		"KeyOutcomeCategory":       tracing.KeyOutcomeCategory,
		"KeyTrafficClass":          tracing.KeyTrafficClass,
		"KeyDependency":            tracing.KeyDependency,
		"KeyOperation":             tracing.KeyOperation,
		"KeyAttempt":               tracing.KeyAttempt,
		"KeyErrorCode":             tracing.KeyErrorCode,
		"KeyInboxAttempt":          tracing.KeyInboxAttempt,
		"KeyInboxDisposition":      tracing.KeyInboxDisposition,
		"KeyInboxGesture":          tracing.KeyInboxGesture,
		"KeyIdempotencyKey":        tracing.KeyIdempotencyKey,
		"KeyIdempotencyKeyDerived": tracing.KeyIdempotencyKeyDerived,
		"KeyIdempotencyKeyInvalid": tracing.KeyIdempotencyKeyInvalid,
		"KeyIdempotencyOutcome":    tracing.KeyIdempotencyOutcome,
		"KeyDeadlineRemainingMS":   tracing.KeyDeadlineRemainingMS,
		"KeyOutboxClaimID":         tracing.KeyOutboxClaimID,
		"KeyOutboxAttempt":         tracing.KeyOutboxAttempt,
		"KeyProcessRole":           tracing.KeyProcessRole,
		"KeyPreviousCategory":      tracing.KeyPreviousCategory,
	}
}

func TestTheVocabularyNamesAreTheSpecNames(t *testing.T) {
	want := map[string]string{
		"KeyCorrelationID":         "dmpf.correlation_id",
		"KeyRequestID":             "dmpf.request_id",
		"KeyTenantID":              "dmpf.tenant_id",
		"KeyOutcomeCategory":       "dmpf.outcome_category",
		"KeyAttempt":               "dmpf.retry.attempt",
		"KeyErrorCode":             "dmpf.error.code",
		"KeyInboxAttempt":          "dmpf.inbox.attempt",
		"KeyInboxDisposition":      "dmpf.inbox.disposition",
		"KeyInboxGesture":          "dmpf.inbox.gesture",
		"KeyIdempotencyKey":        "dmpf.idempotency_key",
		"KeyIdempotencyKeyDerived": "dmpf.idempotency_key.derived",
		"KeyIdempotencyKeyInvalid": "dmpf.idempotency_key.invalid",
		"KeyIdempotencyOutcome":    "dmpf.idempotency_outcome",
		"KeyDeadlineRemainingMS":   "dmpf.deadline.remaining_ms",
		"KeyOutboxClaimID":         "dmpf.outbox.claim_id",
		"KeyOutboxAttempt":         "dmpf.outbox.attempt",
		"KeyProcessRole":           "dmpf.process.role",
	}
	declared := vocabulary()
	for name, literal := range want {
		if got := declared[name]; got != literal {
			t.Errorf("tracing.%s = %q, want %q", name, got, literal)
		}
	}
}

func TestTheVocabularyKeysAreUniqueAndCanonical(t *testing.T) {
	seen := map[string]string{}
	for name, key := range vocabulary() {
		if !canonicalKey(key) {
			t.Errorf("tracing.%s = %q, neither dmpf.* nor a semconv v1.43.0 constant", name, key)
		}
		if other, dup := seen[key]; dup {
			t.Errorf("tracing.%s and tracing.%s share the key %q", name, other, key)
		}
		seen[key] = name
	}
}

func TestTheProcessRoleIsTheResourceKey(t *testing.T) {
	if tracing.KeyProcessRole != otelboot.ProcessRoleAttribute {
		t.Fatalf("tracing.KeyProcessRole = %q, otelboot.ProcessRoleAttribute = %q: one vocabulary", tracing.KeyProcessRole, otelboot.ProcessRoleAttribute)
	}
}

func TestTheBuilderSetIsClosed(t *testing.T) {
	want := map[string]string{
		"CorrelationID":   tracing.KeyCorrelationID,
		"RequestID":       tracing.KeyRequestID,
		"TenantID":        tracing.KeyTenantID,
		"OutcomeCategory": tracing.KeyOutcomeCategory,
		"TrafficClass":    tracing.KeyTrafficClass,
		"Dependency":      tracing.KeyDependency,
		"Attempt":         tracing.KeyAttempt,
		"OutboxClaimID":   tracing.KeyOutboxClaimID,
		"OutboxAttempt":   tracing.KeyOutboxAttempt,

		"MessagingDestinationPartitionID": string(semconv.MessagingDestinationPartitionIDKey),
		"MessagingKafkaOffset":            string(semconv.MessagingKafkaOffsetKey),
	}

	got := builtKeys(t)
	for method, keys := range got {
		key, declared := want[method]
		if !declared {
			t.Errorf("%s is a builder outside the closed set: declare its key here (TRC-04)", method)
			continue
		}
		if len(keys) != 1 || keys[0] != key {
			t.Errorf("%s produces %v, want only %q", method, keys, key)
		}
	}
	for method := range want {
		if _, built := got[method]; !built {
			t.Errorf("%s is declared but Attributes has no such builder", method)
		}
	}
}

func TestTheOutboxAttemptHasItsOwnKey(t *testing.T) {
	got := tracing.Attributes{}.OutboxAttempt(1).KeyValues()

	if len(got) != 1 || string(got[0].Key) != tracing.KeyOutboxAttempt || got[0].Value.AsInt64() != 1 {
		t.Fatalf("KeyValues() = %v, want %s=1: the relay counts its claims under a key of its own", got, tracing.KeyOutboxAttempt)
	}
	if tracing.KeyOutboxAttempt == tracing.KeyAttempt {
		t.Fatalf("%s shares the key of the retry attempt", tracing.KeyOutboxAttempt)
	}
}

// builtKeys calls every builder, whatever its parameter type, with a non-zero
// value and returns, per method, the keys it produced.
func builtKeys(t *testing.T) map[string][]string {
	t.Helper()

	values := map[reflect.Type]reflect.Value{
		reflect.TypeOf(""): reflect.ValueOf("value"),
		reflect.TypeOf(0):  reflect.ValueOf(1),
	}
	attributes := reflect.TypeOf(tracing.Attributes{})
	built := map[string][]string{}
	for i := range attributes.NumMethod() {
		method := attributes.Method(i)
		signature := method.Type
		if signature.NumOut() != 1 || signature.Out(0) != attributes {
			continue
		}
		if signature.NumIn() != 2 {
			t.Errorf("%s takes %d arguments, an arity outside the closed set of builders (TRC-04)", method.Name, signature.NumIn()-1)
			continue
		}
		value, known := values[signature.In(1)]
		if !known {
			t.Errorf("%s takes %v, a type outside the closed set of builder parameters (TRC-04)", method.Name, signature.In(1))
			value = reflect.New(signature.In(1)).Elem()
		}
		out := method.Func.Call([]reflect.Value{reflect.ValueOf(tracing.Attributes{}), value})
		built[method.Name] = nil
		for _, kv := range out[0].Interface().(tracing.Attributes).KeyValues() {
			built[method.Name] = append(built[method.Name], string(kv.Key))
		}
	}
	if len(built) == 0 {
		t.Fatal("reflection found no builder on Attributes")
	}
	return built
}

func TestTheBuilderAccumulatesInOrder(t *testing.T) {
	got := tracing.Attributes{}.
		CorrelationID("c-1").
		RequestID("r-1").
		TrafficClass("write").
		KeyValues()

	want := []attribute.KeyValue{
		attribute.String(tracing.KeyCorrelationID, "c-1"),
		attribute.String(tracing.KeyRequestID, "r-1"),
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
	base := tracing.Attributes{}.TenantID("acme")

	first := base.RequestID("r-1")
	second := base.RequestID("r-2")

	if len(base.KeyValues()) != 1 {
		t.Fatalf("the base set grew to %v: the builder must return a new value", base.KeyValues())
	}
	if first.KeyValues()[1].Value.AsString() != "r-1" {
		t.Fatalf("the first branch reads %v, want r-1", first.KeyValues())
	}
	if second.KeyValues()[1].Value.AsString() != "r-2" {
		t.Fatalf("the second branch reads %v, want r-2", second.KeyValues())
	}
}

func TestKeyValuesReturnsACopy(t *testing.T) {
	attributes := tracing.Attributes{}.TenantID("acme")

	attributes.KeyValues()[0] = attribute.String("tampered", "x")

	if got := string(attributes.KeyValues()[0].Key); got != tracing.KeyTenantID {
		t.Fatalf("key = %q after rewriting the returned slice, want %q", got, tracing.KeyTenantID)
	}
}
