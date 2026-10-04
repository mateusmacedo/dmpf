package otelboot_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func withBaggage(t *testing.T, ctx context.Context, members map[string]string) context.Context {
	t.Helper()
	bag := baggage.FromContext(ctx)
	for key, value := range members {
		member, err := baggage.NewMemberRaw(key, value)
		if err != nil {
			t.Fatalf("NewMemberRaw(%s) = %v", key, err)
		}
		if bag, err = bag.SetMember(member); err != nil {
			t.Fatalf("SetMember(%s) = %v", key, err)
		}
	}
	return baggage.ContextWithBaggage(ctx, bag)
}

var executionMembers = map[string]string{
	tracing.KeyCorrelationID: "corr-1",
	tracing.KeyRequestID:     "req-1",
	tracing.KeyTenantID:      "tenant-1",
}

func spanAttributes(t *testing.T, exporter *tracetest.InMemoryExporter, name string) map[attribute.Key]attribute.Value {
	t.Helper()
	for _, span := range exporter.GetSpans() {
		if span.Name == name {
			return attributesOf(span)
		}
	}
	t.Fatalf("no span %s was exported", name)
	return nil
}

func TestALocalChildInheritsTheExecutionBaggage(t *testing.T) {
	runtime, exporter := startedRuntime(t, nil)

	ctx := withBaggage(t, context.Background(), executionMembers)
	ctx, parent := runtime.Tracer().Start(ctx, "parent")
	_, child := runtime.Tracer().Start(ctx, "child")
	child.End()
	parent.End()
	if err := runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	for _, name := range []string{"parent", "child"} {
		attributes := spanAttributes(t, exporter, name)
		for key, want := range executionMembers {
			if got := attributes[attribute.Key(key)].AsString(); got != want {
				t.Errorf("%s: %s = %q, want %q", name, key, got, want)
			}
		}
	}
}

func TestAMemberOutsideTheFilterIsNotCopied(t *testing.T) {
	runtime, exporter := startedRuntime(t, nil)

	ctx := withBaggage(t, context.Background(), map[string]string{"user.email": "a@b.c", "dmpf.other": "x"})
	_, span := runtime.Tracer().Start(ctx, "orders.place")
	span.End()
	if err := runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	attributes := spanAttributes(t, exporter, "orders.place")
	for _, key := range []attribute.Key{"user.email", "dmpf.other"} {
		if _, present := attributes[key]; present {
			t.Errorf("%s was copied from the baggage, want only the closed set", key)
		}
	}
}

func TestASpanOpenedBeforeTheBaggageDoesNotCarryIt(t *testing.T) {
	runtime, exporter := startedRuntime(t, nil)

	ctx, early := runtime.Tracer().Start(context.Background(), "early")
	ctx = withBaggage(t, ctx, executionMembers)
	_, late := runtime.Tracer().Start(ctx, "late")
	late.End()
	early.End()
	if err := runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	if _, present := spanAttributes(t, exporter, "early")[tracing.KeyCorrelationID]; present {
		t.Error("the span opened before the baggage carries it")
	}
	if got := spanAttributes(t, exporter, "late")[tracing.KeyCorrelationID].AsString(); got != "corr-1" {
		t.Errorf("the span opened after the baggage has %s = %q, want corr-1", tracing.KeyCorrelationID, got)
	}
}

func TestALogRecordReceivesTheExecutionBaggage(t *testing.T) {
	exporter := &recordingExporter{}
	config := validConfig()
	provider := otelboot.NewLoggerProvider(config, exporter)

	var record log.Record
	record.SetBody(attribute.StringValue("placed"))
	ctx := withBaggage(t, context.Background(), map[string]string{
		tracing.KeyCorrelationID: "corr-1", tracing.KeyRequestID: "req-1", tracing.KeyTenantID: "tenant-1", "user.email": "a@b.c",
	})
	provider.Logger("test").Emit(ctx, record)
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	if len(exporter.records) != 1 {
		t.Fatalf("exported %d records, want 1", len(exporter.records))
	}
	got := map[string]string{}
	exporter.records[0].WalkAttributes(func(kv attribute.KeyValue) bool {
		got[string(kv.Key)] = kv.Value.AsString()
		return true
	})
	for key, want := range executionMembers {
		if got[key] != want {
			t.Errorf("record %s = %q, want %q", key, got[key], want)
		}
	}
	if _, present := got["user.email"]; present {
		t.Error("the record received a baggage member outside the closed set")
	}
}

func TestOnlyTheTraceContextPropagatorMayBeDeclared(t *testing.T) {
	for value, refused := range map[string]bool{"": false, "tracecontext": false, "b3": true, "tracecontext,baggage": true, "none": true} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("OTEL_PROPAGATORS", value)
			err := validConfig().Validate()
			if refused && !errors.Is(err, otelboot.ErrPropagatorNotW3C) {
				t.Fatalf("Validate() with OTEL_PROPAGATORS=%q = %v, want ErrPropagatorNotW3C", value, err)
			}
			if !refused && err != nil {
				t.Fatalf("Validate() with OTEL_PROPAGATORS=%q = %v, want nil", value, err)
			}
		})
	}
}
