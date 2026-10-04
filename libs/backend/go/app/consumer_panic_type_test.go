package app_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const keyPanicType = "dmpf.panic.type"

func panickingWithAnError(context.Context, ports.Receipt, envelope.Envelope) (application.Disposition, error) {
	panic(errors.New(handlerPanic))
}

func indexingPastTheRows(context.Context, ports.Receipt, envelope.Envelope) (application.Disposition, error) {
	var rows []int
	return application.R1D1, errors.New(strconv.Itoa(rows[len(handlerPanic)]))
}

func requireOnlyThePanicType(t *testing.T, where string, attributes map[string]attribute.Value, want string, leak string) {
	t.Helper()
	if got, ok := attributes[keyPanicType]; !ok || got != attribute.StringValue(want) {
		t.Errorf("%s %s = %v (present %v), want %q: the Go type of the recovered value is the only diagnosis left", where, keyPanicType, got.String(), ok, want)
	}
	for key, value := range attributes {
		if strings.Contains(value.String(), leak) {
			t.Errorf("%s %s = %q, want no panic value (ERR-20)", where, key, value.String())
		}
	}
}

func TestAPanickingHandlerLeavesOnlyTheGoTypeOfTheValue(t *testing.T) {
	for _, tc := range []struct {
		name   string
		handle app.Handler
		want   string
	}{
		{name: "a string", handle: panickingHandler, want: "string"},
		{name: "an error", handle: panickingWithAnError, want: "*errors.errorString"},
		{name: "a runtime error", handle: indexingPastTheRows, want: "runtime.boundsError"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := validRaw(t)
			consumer, recorder, collect := loggedConsumer(t, &fakeHandler{}, &fakeContainment{})
			consumer.Handle = tc.handle

			_, _ = consumeLikeTheWorker(t, context.Background(), consumer, app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{})

			requireOnlyThePanicType(t, messageConsumed, recordAttributes(onlyConsumed(t, collect())), tc.want, handlerPanicLeak)
			process := onlyProcess(t, recorder)
			requireOnlyThePanicType(t, process.Name(), spanAttributes(process), tc.want, handlerPanicLeak)
		})
	}
}

func TestAPanicInTheContainmentOrTheGestureLeavesOnlyTheGoTypeOfTheValue(t *testing.T) {
	rejection := application.NewFailure(application.DomainRejection, false, errHandler)
	transient := application.NewFailure(application.TransientDependency, true, errHandler)
	for _, tc := range []struct {
		name       string
		handler    *fakeHandler
		quarantine bool
		ack        *panickingAck
	}{
		{name: "the quarantine panics", handler: &fakeHandler{disposition: application.R1D4, err: rejection}, quarantine: true, ack: &panickingAck{}},
		{name: "the ack panics", handler: &fakeHandler{disposition: application.R1D1}, ack: &panickingAck{onAck: true}},
		{name: "the release panics", handler: &fakeHandler{disposition: application.R1D3, err: transient}, ack: &panickingAck{onRelease: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := validRaw(t)
			var containment ports.Containment = &fakeContainment{}
			if tc.quarantine {
				containment = panickingContainment{}
			}
			consumer, recorder, collect := loggedConsumer(t, tc.handler, containment)

			_, _ = consumeLikeTheWorker(t, context.Background(), consumer, app.Delivery{Raw: raw, Attempt: 1}, tc.ack)

			requireOnlyThePanicType(t, messageConsumed, recordAttributes(onlyConsumed(t, collect())), "string", gesturePanicLeak)
			process := onlyProcess(t, recorder)
			requireOnlyThePanicType(t, process.Name(), spanAttributes(process), "string", gesturePanicLeak)
		})
	}
}

func TestAFailureWithoutAPanicCarriesNoPanicType(t *testing.T) {
	raw, _ := validRaw(t)
	consumer, recorder, collect := loggedConsumer(t, returnedUnexpected(), &fakeContainment{})

	_, _ = consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{})

	if got, ok := recordAttributes(onlyConsumed(t, collect()))[keyPanicType]; ok {
		t.Errorf("%q %s = %v, want it absent: nothing panicked", messageConsumed, keyPanicType, got.String())
	}
	requireAbsent(t, onlyProcess(t, recorder), keyPanicType)
}
