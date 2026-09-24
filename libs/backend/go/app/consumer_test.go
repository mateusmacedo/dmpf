package app_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	consumerName = "reservations"
	occurred     = ports.Instant(1_756_000_000_000_000_000)
)

var errHandler = errors.New("handler failed")

type fixedClock struct{}

func (fixedClock) Now() ports.Instant { return occurred }

type fakeHandler struct {
	disposition application.Disposition
	err         error
	calls       int
	receipt     ports.Receipt
	env         envelope.Envelope
	mc          ports.MessageContext
	mcPresent   bool
	execution   ports.ExecutionContext
	executed    bool
	trace       *[]string
}

func (h *fakeHandler) handle(ctx context.Context, r ports.Receipt, env envelope.Envelope) (application.Disposition, error) {
	h.calls++
	h.receipt = r
	h.env = env
	h.mc, h.mcPresent = ports.MessageContextFrom(ctx)
	h.execution, h.executed = ports.ExecutionContextFrom(ctx)
	if h.trace != nil {
		*h.trace = append(*h.trace, "handle")
	}
	return h.disposition, h.err
}

type fakeAck struct {
	acks     int
	releases int
	err      error
	trace    *[]string
}

func (a *fakeAck) Ack(context.Context) error {
	a.acks++
	if a.trace != nil {
		*a.trace = append(*a.trace, "ack")
	}
	return a.err
}

func (a *fakeAck) Release(context.Context) error {
	a.releases++
	if a.trace != nil {
		*a.trace = append(*a.trace, "release")
	}
	return a.err
}

type fakeContainment struct {
	contained []ports.Contained
	err       error
}

func (c *fakeContainment) Quarantine(_ context.Context, item ports.Contained) error {
	c.contained = append(c.contained, item)
	return c.err
}

func validRaw(t *testing.T) ([]byte, envelope.Envelope) {
	t.Helper()
	payload, typeURL, err := envelope.Pack(&eventv1.OrderPlaced{OrderId: "o-1", ItemCount: 2})
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	env := envelope.Envelope{
		ID:              "evt-1",
		Source:          "urn:dmpf:orders",
		SpecVersion:     envelope.SpecVersion,
		Type:            "com.company.orders.order-placed.v1",
		Subject:         "order/o-1",
		Time:            timestamppb.New(time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)),
		DataSchema:      typeURL,
		DataContentType: envelope.ContentType,
		CorrelationID:   "corr-1",
		CausationID:     "evt-0",
		PartitionKey:    "o-1",
		TraceParent:     "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		TenantID:        &fixtureTenant,
		Payload:         payload,
	}
	return encode(t, env), env
}

var fixtureTenant = "acme"

func encode(t *testing.T, env envelope.Envelope) []byte {
	t.Helper()
	ce, err := envelope.Encode(env)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(ce)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return raw
}

func newConsumer(h *fakeHandler, c *fakeContainment, maxAttempts int) app.Consumer {
	return app.Consumer{
		Name:        consumerName,
		MaxAttempts: maxAttempts,
		Handle:      h.handle,
		Containment: c,
		Clock:       fixedClock{},
		Timeout:     testTimeout,
		Boundary:    testBoundary,
		Locale:      "en",
	}
}

// testBoundary trusts the producer validRaw declares, so every other test
// exercises what happens after the boundary admitted the message.
var testBoundary = app.Boundary{Transport: app.TransportDevelopmentOnly, Sources: []string{"urn:dmpf:orders"}}

// testTimeout is the time policy the suite declares. CTX-28 makes it the
// consumer's own, so the adapter refuses to run without one.
const testTimeout = 5 * time.Second

func TestInvalidEnvelopeIsContainedBeforeAnyHandling(t *testing.T) {
	t.Parallel()
	handler := &fakeHandler{}
	containment := &fakeContainment{}
	ack := &fakeAck{}
	raw := []byte("not a cloudevent")

	outcome, err := newConsumer(handler, containment, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, ack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler.calls != 0 {
		t.Fatalf("handler must not run on an invalid envelope (INB-10), ran %d times", handler.calls)
	}
	if !outcome.Contained || outcome.Reason != ports.ReasonInvalidEnvelope || outcome.Classified {
		t.Fatalf("outcome = %+v, want contained as invalid-envelope and unclassified", outcome)
	}
	if len(containment.contained) != 1 || !bytes.Equal(containment.contained[0].Envelope, raw) {
		t.Fatalf("quarantine must keep the raw bytes, got %+v", containment.contained)
	}
	if containment.contained[0].Consumer != consumerName || containment.contained[0].At != occurred {
		t.Fatalf("contained = %+v", containment.contained[0])
	}
	if ack.acks != 1 || ack.releases != 0 {
		t.Fatalf("ack=%d release=%d, want the message taken out of the flow", ack.acks, ack.releases)
	}
}

// CTX-27 and IDN-04: an intact envelope from a producer outside the declared
// boundary produces no context, so it never reaches the handler or the inbox.
func TestAnEnvelopeFromOutsideTheBoundaryIsContainedBeforeAnyHandling(t *testing.T) {
	t.Parallel()
	raw, env := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}
	containment := &fakeContainment{}
	ack := &fakeAck{}
	consumer := newConsumer(handler, containment, 3)
	consumer.Boundary.Sources = []string{"urn:dmpf:someone-else"}

	outcome, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, ack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler.calls != 0 {
		t.Fatalf("handler ran %d times on a message outside the boundary; it must not produce a context", handler.calls)
	}
	if !outcome.Contained || outcome.Reason != ports.ReasonUntrustedBoundary || outcome.Classified {
		t.Fatalf("outcome = %+v, want contained as untrusted-boundary and unclassified", outcome)
	}
	if len(containment.contained) != 1 || !bytes.Equal(containment.contained[0].Envelope, raw) {
		t.Fatalf("quarantine must keep the raw bytes, got %+v", containment.contained)
	}
	if got := containment.contained[0].MessageID; got != ports.MessageID(env.ID) {
		t.Fatalf("MessageID = %q, want %q: the envelope was intact, so its id is known", got, env.ID)
	}
	if ack.acks != 1 || ack.releases != 0 {
		t.Fatalf("ack=%d release=%d, want the message taken out of the flow", ack.acks, ack.releases)
	}
}

// CTX-24..CTX-28 in one place: the adapter rebuilds the context from the
// envelope, so no consumer has to remember to (IDN-14's argument, applied to
// the consumption edge).
func TestTheHandlerReceivesTheContextRebuiltFromTheEnvelope(t *testing.T) {
	t.Parallel()
	raw, env := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}

	if _, err := newConsumer(handler, &fakeContainment{}, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handler.executed {
		t.Fatal("the handler received no execution context (CTX-24)")
	}
	ec := handler.execution
	if ec.CorrelationID() != env.CorrelationID || ec.TraceContext() != env.TraceParent {
		t.Fatalf("correlation %q, trace %q; want the envelope's %q, %q", ec.CorrelationID(), ec.TraceContext(), env.CorrelationID, env.TraceParent)
	}
	if causation, ok := ec.CausationID(); !ok || causation != env.CausationID {
		t.Fatalf("causation = %q, %v; want the envelope's %q", causation, ok, env.CausationID)
	}
	if tenant, ok := ec.Tenant(); !ok || string(tenant) != *env.TenantID {
		t.Fatalf("tenant = %q, %v; want the envelope's %q (CTX-24)", tenant, ok, *env.TenantID)
	}
	if _, ok := ec.Subject(); ok {
		t.Fatal("a subject was rebuilt from the envelope: provenance is not identity (CTX-25)")
	}
	if ec.RequestID() == "" || ec.RequestID() == env.ID {
		t.Fatalf("request_id = %q, want the consumer's own per attempt, never read from the envelope (CTX-28)", ec.RequestID())
	}
	if ec.Deadline() <= 0 || ec.Locale() != "en" {
		t.Fatalf("deadline %d, locale %q; want the consumer's own policy", ec.Deadline(), ec.Locale())
	}
}

// CTX-26: an envelope without tenant is a platform chain, and the context
// carries the absence; no default is put in its place.
func TestAnEnvelopeWithoutTenantRebuildsAContextWithoutTenant(t *testing.T) {
	t.Parallel()
	_, env := validRaw(t)
	env.TenantID = nil
	raw := encode(t, env)
	handler := &fakeHandler{disposition: application.R1D1}

	if _, err := newConsumer(handler, &fakeContainment{}, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tenant, ok := handler.execution.Tenant(); ok {
		t.Fatalf("tenant = %q, want absent (CTX-26, IDN-20)", tenant)
	}
}

func TestHandlerReceivesTheReceiptDerivedFromTheEnvelope(t *testing.T) {
	t.Parallel()
	raw, env := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}
	containment := &fakeContainment{}
	ack := &fakeAck{}

	outcome, err := newConsumer(handler, containment, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, ack)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler.calls != 1 {
		t.Fatalf("handler calls = %d", handler.calls)
	}
	want := ports.Receipt{
		Consumer:    consumerName,
		MessageID:   ports.MessageID(env.ID),
		MessageType: env.Type,
		PayloadHash: payloadhash.Sum(env.Payload),
		ReceivedAt:  occurred,
	}
	if handler.receipt != want {
		t.Fatalf("receipt = %+v, want %+v", handler.receipt, want)
	}
	if !bytes.Equal(handler.env.Payload, env.Payload) {
		t.Fatal("handler must see the transported payload bytes")
	}
	if outcome.Disposition != application.R1D1 || outcome.Contained || !outcome.Classified {
		t.Fatalf("outcome = %+v", outcome)
	}
	if ack.acks != 1 || ack.releases != 0 || len(containment.contained) != 0 {
		t.Fatalf("R1×D1 must only ack: ack=%d release=%d contained=%d", ack.acks, ack.releases, len(containment.contained))
	}
}

// The adapter is where the consumed message becomes the cause of whatever the
// handler emits (FND-07 §8.6 item 3): the three ENV-08 attributes travel from
// the envelope to the context before the handler runs.
func TestHandlerReceivesTheMessageContextOfTheEnvelope(t *testing.T) {
	t.Parallel()
	raw, env := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}

	if _, err := newConsumer(handler, &fakeContainment{}, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handler.mcPresent {
		t.Fatal("the handler received no message context (FND-07 §8.6 item 3)")
	}
	want := ports.MessageContext{CorrelationID: env.CorrelationID, CausationID: env.ID, Traceparent: env.TraceParent}
	if handler.mc != want {
		t.Fatalf("message context = %+v, want %+v — causation is the consumed message, not its own cause", handler.mc, want)
	}
}

func TestConfirmingDispositionsOnlyAck(t *testing.T) {
	t.Parallel()
	for _, disposition := range []application.Disposition{application.R1D2, application.R2, application.R3} {
		t.Run(disposition.String(), func(t *testing.T) {
			t.Parallel()
			raw, _ := validRaw(t)
			handler := &fakeHandler{disposition: disposition}
			containment := &fakeContainment{}
			ack := &fakeAck{}

			outcome, err := newConsumer(handler, containment, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, ack)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if outcome.Disposition != disposition || outcome.Contained {
				t.Fatalf("outcome = %+v", outcome)
			}
			if ack.acks != 1 || ack.releases != 0 || len(containment.contained) != 0 {
				t.Fatalf("ack=%d release=%d contained=%d", ack.acks, ack.releases, len(containment.contained))
			}
		})
	}
}

func TestTransientFailureIsReleasedWhileAttemptsRemain(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D3, err: errHandler}
	containment := &fakeContainment{}
	ack := &fakeAck{}

	outcome, err := newConsumer(handler, containment, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 2}, ack)
	if !errors.Is(err, errHandler) {
		t.Fatalf("the handler error must surface, got %v", err)
	}
	if outcome.Disposition != application.R1D3 || outcome.Contained {
		t.Fatalf("outcome = %+v", outcome)
	}
	if ack.releases != 1 || ack.acks != 0 || len(containment.contained) != 0 {
		t.Fatalf("R1×D3 must release: ack=%d release=%d contained=%d", ack.acks, ack.releases, len(containment.contained))
	}
}

func TestTransientFailureIsContainedWhenAttemptsAreExhausted(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	cause := application.NewFailure(application.TransientDependency, true, errHandler)
	handler := &fakeHandler{disposition: application.R1D3, err: cause}
	containment := &fakeContainment{}
	ack := &fakeAck{}

	outcome, err := newConsumer(handler, containment, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 3}, ack)
	if !errors.Is(err, errHandler) {
		t.Fatalf("the handler error must surface, got %v", err)
	}
	if !outcome.Contained || outcome.Reason != ports.ReasonAttemptsExhausted {
		t.Fatalf("outcome = %+v, want contained as attempts-exhausted (GAR-08)", outcome)
	}
	if len(containment.contained) != 1 || containment.contained[0].Error != string(application.TransientDependency) {
		t.Fatalf("contained = %+v, want the category as the sanitized error", containment.contained)
	}
	if ack.acks != 1 || ack.releases != 0 {
		t.Fatalf("a contained message leaves the flow: ack=%d release=%d", ack.acks, ack.releases)
	}
}

func TestZeroMaxAttemptsNeverContainsForAttempts(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D3, err: errHandler}
	containment := &fakeContainment{}
	ack := &fakeAck{}

	if _, err := newConsumer(handler, containment, 0).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 99}, ack); !errors.Is(err, errHandler) {
		t.Fatalf("got %v", err)
	}
	if ack.releases != 1 || len(containment.contained) != 0 {
		t.Fatalf("release=%d contained=%d", ack.releases, len(containment.contained))
	}
}

func TestTerminalFailureAndCollisionAreContained(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		disposition application.Disposition
		err         error
		wantReason  ports.Reason
		wantError   string
	}{
		{"terminal failure", application.R1D4, application.NewFailure(application.Validation, false, errHandler), ports.ReasonTerminalFailure, string(application.Validation)},
		{"terminal failure without category", application.R1D4, errHandler, ports.ReasonTerminalFailure, application.R1D4.String()},
		{"collision", application.R4, nil, ports.ReasonCollision, application.R4.String()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, env := validRaw(t)
			handler := &fakeHandler{disposition: tc.disposition, err: tc.err}
			containment := &fakeContainment{}
			ack := &fakeAck{}

			outcome, err := newConsumer(handler, containment, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, ack)
			if tc.err != nil && !errors.Is(err, errHandler) {
				t.Fatalf("got %v", err)
			}
			if tc.err == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !outcome.Contained || outcome.Reason != tc.wantReason || outcome.Disposition != tc.disposition {
				t.Fatalf("outcome = %+v", outcome)
			}
			if len(containment.contained) != 1 {
				t.Fatalf("contained = %d", len(containment.contained))
			}
			got := containment.contained[0]
			if got.Reason != tc.wantReason || got.Error != tc.wantError || got.MessageID != ports.MessageID(env.ID) || !bytes.Equal(got.Envelope, raw) {
				t.Fatalf("contained = %+v", got)
			}
			if ack.acks != 1 || ack.releases != 0 {
				t.Fatalf("ack=%d release=%d", ack.acks, ack.releases)
			}
		})
	}
}

func TestBrokerEffectHappensAfterTheHandlerReturns(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	var trace []string
	handler := &fakeHandler{disposition: application.R1D1, trace: &trace}
	ack := &fakeAck{trace: &trace}

	if _, err := newConsumer(handler, &fakeContainment{}, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, ack); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trace) != 2 || trace[0] != "handle" || trace[1] != "ack" {
		t.Fatalf("trace = %v, want the ack strictly after the handler (INB-08)", trace)
	}
}

func TestQuarantineKeepsBytesARemarshalWouldDrop(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	// Field 999, varint 1: a tag the CloudEvent schema does not declare, so
	// proto.Unmarshal keeps it as unknown and a re-marshal would drop it.
	raw = append(append([]byte{}, raw...), 0xB8, 0x3E, 0x01)
	handler := &fakeHandler{disposition: application.R4}
	containment := &fakeContainment{}

	if _, err := newConsumer(handler, containment, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler.calls != 1 {
		t.Fatal("an unknown field must not invalidate the envelope")
	}
	if len(containment.contained) != 1 || !bytes.Equal(containment.contained[0].Envelope, raw) {
		t.Fatal("quarantine must persist the transported bytes, unknown field included (GAR-07)")
	}
}

func TestBrokerFailureSurfaces(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	brokerErr := errors.New("broker down")
	handler := &fakeHandler{disposition: application.R1D1}
	ack := &fakeAck{err: brokerErr}

	outcome, err := newConsumer(handler, &fakeContainment{}, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, ack)
	if !errors.Is(err, brokerErr) {
		t.Fatalf("got %v", err)
	}
	if outcome.Disposition != application.R1D1 {
		t.Fatalf("the disposition is still reported: %+v", outcome)
	}
}

func TestContainmentFailureSurfacesAndDoesNotAck(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	quarantineErr := errors.New("quarantine down")
	handler := &fakeHandler{disposition: application.R4}
	ack := &fakeAck{}

	if _, err := newConsumer(handler, &fakeContainment{err: quarantineErr}, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, ack); !errors.Is(err, quarantineErr) {
		t.Fatalf("got %v", err)
	}
	if ack.acks != 0 {
		t.Fatal("a message that could not be contained must not be confirmed (GAR-07: contained, not lost)")
	}
}

func TestConsumerRequiresItsCollaborators(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	cases := map[string]app.Consumer{
		"name":        {MaxAttempts: 1, Handle: (&fakeHandler{}).handle, Containment: &fakeContainment{}, Clock: fixedClock{}, Timeout: testTimeout, Boundary: testBoundary, Locale: "en"},
		"handler":     {Name: consumerName, Containment: &fakeContainment{}, Clock: fixedClock{}, Timeout: testTimeout, Boundary: testBoundary, Locale: "en"},
		"containment": {Name: consumerName, Handle: (&fakeHandler{}).handle, Clock: fixedClock{}, Timeout: testTimeout, Boundary: testBoundary, Locale: "en"},
		"clock":       {Name: consumerName, Handle: (&fakeHandler{}).handle, Containment: &fakeContainment{}, Timeout: testTimeout, Boundary: testBoundary, Locale: "en"},
		"timeout":     {Name: consumerName, Handle: (&fakeHandler{}).handle, Containment: &fakeContainment{}, Clock: fixedClock{}, Boundary: testBoundary, Locale: "en"},
		"boundary":    {Name: consumerName, Handle: (&fakeHandler{}).handle, Containment: &fakeContainment{}, Clock: fixedClock{}, Timeout: testTimeout, Locale: "en"},
		"locale":      {Name: consumerName, Handle: (&fakeHandler{}).handle, Containment: &fakeContainment{}, Clock: fixedClock{}, Timeout: testTimeout, Boundary: testBoundary},
		"boundary without sources": {Name: consumerName, Handle: (&fakeHandler{}).handle, Containment: &fakeContainment{}, Clock: fixedClock{}, Timeout: testTimeout,
			Boundary: app.Boundary{Transport: app.TransportVerified}, Locale: "en"},
		"boundary with an undeclared transport": {Name: consumerName, Handle: (&fakeHandler{}).handle, Containment: &fakeContainment{}, Clock: fixedClock{}, Timeout: testTimeout,
			Boundary: app.Boundary{Transport: "trusted-because-internal", Sources: []string{"urn:dmpf:orders"}}, Locale: "en"},
	}
	for name, consumer := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := consumer.Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, &fakeAck{})
			if !errors.Is(err, app.ErrIncompleteConsumer) {
				t.Fatalf("got %v, want ErrIncompleteConsumer", err)
			}
		})
	}
}

func TestInvalidEnvelopeErrorIsSanitized(t *testing.T) {
	t.Parallel()
	containment := &fakeContainment{}

	if _, err := newConsumer(&fakeHandler{}, containment, 3).Consume(context.Background(), app.Delivery{Raw: []byte{0xff, 0xfe, 0xfd}, Attempt: 1}, &fakeAck{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(containment.contained) != 1 {
		t.Fatalf("contained = %d", len(containment.contained))
	}
	if got := containment.contained[0].Error; got != envelope.ErrMalformed.Error() {
		t.Fatalf("Error = %q, want the sentinel alone, without the wire decoder's message (ERR-20)", got)
	}
}

func TestUnknownDispositionIsAnErrorNotAPanic(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.Disposition(99), err: errHandler}
	containment := &fakeContainment{}
	ack := &fakeAck{}

	outcome, err := newConsumer(handler, containment, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 1}, ack)
	if !errors.Is(err, app.ErrUnknownDisposition) || !errors.Is(err, errHandler) {
		t.Fatalf("err = %v, want ErrUnknownDisposition joined with the handler's error", err)
	}
	if outcome != (app.Outcome{}) {
		t.Fatalf("outcome = %+v, want the zero value", outcome)
	}
	if ack.acks != 0 || ack.releases != 0 || len(containment.contained) != 0 {
		t.Fatal("a defective disposition must leave the message untouched: no ack, no release, no containment")
	}
}

// CTX-11, retry column for subject and permissions: §3.6 reconstructs the
// consumption with the consumer's own workload identity (CTX-25), so what the
// producer's subject could do never reaches the decision on this side.
func TestTheConsumerActsWithoutTheProducersIdentity(t *testing.T) {
	t.Parallel()
	raw, _ := validRaw(t)
	handler := &fakeHandler{disposition: application.R1D1}

	if _, err := newConsumer(handler, &fakeContainment{}, 3).Consume(context.Background(), app.Delivery{Raw: raw, Attempt: 2}, &fakeAck{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subject, ok := handler.execution.Subject(); ok {
		t.Fatalf("subject = %q on a redelivery; provenance is not identity (CTX-25)", subject)
	}
	if permissions := handler.execution.Permissions(); permissions != nil {
		t.Fatalf("permissions = %v on a redelivery; none is carried from the producer (CTX-12)", permissions)
	}
}
