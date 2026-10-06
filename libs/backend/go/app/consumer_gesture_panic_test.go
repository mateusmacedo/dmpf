package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	gesturePanic     = "broker: lease of tenant acme lost at 10.0.0.8"
	gesturePanicLeak = "10.0.0.8"
)

type panickingContainment struct{}

func (panickingContainment) Quarantine(context.Context, ports.Contained) error { panic(gesturePanic) }

type panickingAck struct {
	fakeAck
	onAck     bool
	onRelease bool
}

func (a *panickingAck) Ack(ctx context.Context) error {
	if a.onAck {
		panic(gesturePanic)
	}
	return a.fakeAck.Ack(ctx)
}

func (a *panickingAck) Release(ctx context.Context) error {
	if a.onRelease {
		panic(gesturePanic)
	}
	return a.fakeAck.Release(ctx)
}

func requireNoLeak(t *testing.T, where string, attributes []attribute.KeyValue) {
	t.Helper()
	for _, kv := range attributes {
		if strings.Contains(kv.Value.String(), gesturePanicLeak) {
			t.Errorf("%s %s = %q, want no panic value (ERR-20)", where, kv.Key, kv.Value.String())
		}
	}
}

func TestAPanicInTheContainmentOrTheGestureFailsTheConsumptionAsUnexpected(t *testing.T) {
	rejection := application.NewFailure(application.DomainRejection, false, errHandler)
	transient := application.NewFailure(application.TransientDependency, true, errHandler)
	unexpected := attribute.StringValue(string(application.Unexpected))

	for _, tc := range []struct {
		name       string
		handler    *fakeHandler
		untrusted  bool
		quarantine bool
		ack        *panickingAck
	}{
		{name: "the quarantine panics", handler: &fakeHandler{disposition: application.R1D4, err: rejection}, quarantine: true, ack: &panickingAck{}},
		{name: "the ack after the quarantine panics", handler: &fakeHandler{disposition: application.R1D4, err: rejection}, ack: &panickingAck{onAck: true}},
		{name: "the ack of a confirming disposition panics", handler: &fakeHandler{disposition: application.R1D1}, ack: &panickingAck{onAck: true}},
		{name: "the release panics", handler: &fakeHandler{disposition: application.R1D3, err: transient}, ack: &panickingAck{onRelease: true}},
		{name: "the quarantine outside the boundary panics", handler: &fakeHandler{}, untrusted: true, quarantine: true, ack: &panickingAck{}},
		{name: "the ack outside the boundary panics", handler: &fakeHandler{}, untrusted: true, ack: &panickingAck{onAck: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := validRaw(t)
			var containment ports.Containment = &fakeContainment{}
			if tc.quarantine {
				containment = panickingContainment{}
			}
			consumer, recorder, collect := loggedConsumer(t, tc.handler, containment)
			reader := sdkmetric.NewManualReader()
			consumer.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			if tc.untrusted {
				consumer.Boundary = app.Boundary{Transport: app.TransportVerified, Sources: []string{"urn:dmpf:billing"}}
			}

			_, err := consumeLikeTheWorker(t, context.Background(), consumer, app.Delivery{Raw: raw, Attempt: 1}, tc.ack)

			if err == nil || strings.Contains(err.Error(), gesturePanicLeak) {
				t.Errorf("Consume() = %v, want an error without the panic value (ERR-20)", err)
			}
			var categorized redact.Categorized
			if !errors.As(err, &categorized) || categorized.ErrorCategory() != string(application.Unexpected) {
				t.Errorf("first category found in Consume() = %v, want %s ahead of the handler's: the worker logs that one (kafka/classifier.go:68-74, sqs/classifier.go:82-88)", categorized, application.Unexpected)
			}
			if tc.handler.err != nil && !errors.Is(err, tc.handler.err) {
				t.Errorf("Consume() = %v, want the handler's cause kept in the tree", err)
			}
			if tc.quarantine && tc.ack.acks != 0 {
				t.Errorf("%d acks after the quarantine panicked, want none: the message was not kept (GAR-07)", tc.ack.acks)
			}

			record := onlyConsumed(t, collect())
			if record.Severity() != log.SeverityError {
				t.Errorf("severity = %v, want error for a consumption that panicked (RF-A5)", record.Severity())
			}
			requireRecord(t, record, map[string]attribute.Value{tracing.KeyOutcomeCategory: unexpected})
			if gesture, ok := recordAttributes(record)[tracing.KeyInboxGesture]; ok {
				t.Errorf("%q %s = %v, want it absent: the gesture did not conclude", messageConsumed, tracing.KeyInboxGesture, gesture.String())
			}
			var logged []attribute.KeyValue
			for key, value := range recordAttributes(record) {
				logged = append(logged, attribute.KeyValue{Key: attribute.Key(key), Value: value})
			}
			requireNoLeak(t, messageConsumed, logged)

			process := onlyProcess(t, recorder)
			if process.Status() != (sdktrace.Status{Code: codes.Error}) {
				t.Errorf("status = %v, want Error without a description (TRC-12)", process.Status())
			}
			requireAttributes(t, process, map[string]attribute.Value{
				string(semconv.ErrorTypeKey): unexpected,
				tracing.KeyOutcomeCategory:   unexpected,
			})
			requireAbsent(t, process, tracing.KeyInboxGesture)
			requireNoLeak(t, process.Name(), process.Attributes())
			for _, event := range process.Events() {
				requireNoLeak(t, process.Name()+" "+event.Name, event.Attributes)
			}

			duration := onlyDuration(t, reader)
			if value, _ := duration.Attributes.Value(semconv.ErrorTypeKey); value.AsString() != string(application.Unexpected) {
				t.Errorf("messaging.process.duration error.type = %q, want %q", value.AsString(), application.Unexpected)
			}
		})
	}
}

func TestAPanicContainingAnInvalidEnvelopeIsAnErrorNotAPanic(t *testing.T) {
	for _, tc := range []struct {
		name        string
		containment ports.Containment
		ack         *panickingAck
	}{
		{name: "the quarantine panics", containment: panickingContainment{}, ack: &panickingAck{}},
		{name: "the ack panics", containment: &fakeContainment{}, ack: &panickingAck{onAck: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			consumer := newConsumer(&fakeHandler{}, &fakeContainment{}, 3)
			consumer.Containment = tc.containment

			_, err := consumeLikeTheWorker(t, context.Background(), consumer, app.Delivery{Raw: []byte("not a cloudevent"), Attempt: 1}, tc.ack)

			if err == nil || strings.Contains(err.Error(), gesturePanicLeak) {
				t.Errorf("Consume() = %v, want an error without the panic value (ERR-20)", err)
			}
			if tc.ack.acks != 0 {
				t.Errorf("%d acks, want none: neither the quarantine nor the ack concluded (GAR-07)", tc.ack.acks)
			}
		})
	}
}
