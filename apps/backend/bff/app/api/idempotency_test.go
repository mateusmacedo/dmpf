package api_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	ordersv1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/service/v1"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type command struct {
	method, path, body, context string
}

var commands = []command{
	{http.MethodPost, "/orders/o-1/items", `{"sku":"A","quantity":1}`, "AddItem"},
	{http.MethodPost, "/orders/o-1/place", "", "PlaceOrder"},
	{http.MethodPost, "/reservations/o-1/reserve", `{"items":1}`, "Reserve"},
	{http.MethodPost, "/reservations/o-1/cancel", "", "Cancel"},
	{http.MethodPost, "/bookings/booking", `{"bookingId":"b-1","resourceId":"room-1","quantity":2}`, "ReserveBooking"},
	{http.MethodPost, "/bookings/booking/b-1/cancel", "", "CancelBooking"},
	{http.MethodPost, "/bookings/resource", `{"code":"room-1"}`, "RegisterResource"},
}

func derived(subject, key string) string {
	digest := sha256.Sum256([]byte(subject + "\x00" + key))
	return hex.EncodeToString(digest[:])
}

func TestTheContextReceivesTheKeyDerivedFromTheSubject(t *testing.T) {
	fake := &fakeContexts{}
	f := newFixture(t, fake)

	f.post(t, "/orders/o-1/items", `{"sku":"A","quantity":1}`)

	calls := fake.callsTo("AddItem")
	if len(calls) != 1 {
		t.Fatalf("AddItem reached the context %d times, want 1", len(calls))
	}
	got := calls[0].md.Get(kernelgrpc.IdempotencyKey)
	if len(got) != 1 || got[0] != derived("tester", "k-1") {
		t.Fatalf("idempotency-key = %v, want hex(sha256(subject 0x00 key)) %s", got, derived("tester", "k-1"))
	}
	if !ports.ValidIdempotencyKey(got[0]) || len(got[0]) != 64 {
		t.Fatalf("derived key %q is not a 64-character key the context accepts", got[0])
	}
}

func TestTwoSubjectsWithTheSameClientKeyNeverShareAnEntry(t *testing.T) {
	fake := &fakeContexts{}
	f := newFixture(t, fake)
	other := strings.Replace(testCredential, `"sub":"tester"`, `"sub":"another"`, 1)

	f.post(t, "/orders/o-1/items", `{"sku":"A","quantity":1}`)
	f.do(t, http.MethodPost, "/orders/o-1/items", strings.NewReader(`{"sku":"A","quantity":1}`),
		"Idempotency-Key", "k-1", "Content-Type", "application/json", "Authorization", other)

	calls := fake.callsTo("AddItem")
	if len(calls) != 2 {
		t.Fatalf("AddItem reached the context %d times, want 2", len(calls))
	}
	if first, second := calls[0].md.Get(kernelgrpc.IdempotencyKey), calls[1].md.Get(kernelgrpc.IdempotencyKey); first[0] == second[0] {
		t.Fatalf("both subjects sent key %q: the second would replay the first one's answer", first[0])
	}
}

func TestAReplayedCommandIsAnsweredWithIdempotentReplayed(t *testing.T) {
	for _, c := range commands {
		t.Run(c.context, func(t *testing.T) {
			fresh := newFixture(t, &fakeContexts{})
			first := fresh.post(t, c.path, c.body)
			replayed := newFixture(t, (&fakeContexts{}).replay(c.context))
			again := replayed.post(t, c.path, c.body)

			if got := first.Header().Get("Idempotent-Replayed"); got != "" {
				t.Fatalf("first answer carries Idempotent-Replayed %q, want none", got)
			}
			if again.Code != first.Code || again.Body.String() != first.Body.String() {
				t.Fatalf("replay = %d %s, want the first answer %d %s", again.Code, again.Body.String(), first.Code, first.Body.String())
			}
			if got := again.Header().Get("Idempotent-Replayed"); got != "true" {
				t.Fatalf("replay carries Idempotent-Replayed %q, want true", got)
			}
		})
	}
}

func TestAReplayedRefusalKeepsIts422AndIsMarked(t *testing.T) {
	fake := (&fakeContexts{}).replay("PlaceOrder").on("PlaceOrder", func(int) (any, error) {
		return &ordersv1.PlaceOrderResponse{Result: &ordersv1.PlaceOrderResponse_Rejection{Rejection: &ordersv1.Rejection{Code: "orders/order-not-open", Message: "the order is not open"}}}, nil
	})
	f := newFixture(t, fake)

	rec := f.post(t, "/orders/o-1/place", "")

	requireRejection(t, rec, http.StatusUnprocessableEntity, "orders/order-not-open")
	if got := rec.Header().Get("Idempotent-Replayed"); got != "true" {
		t.Fatalf("Idempotent-Replayed = %q, want true on the replayed refusal", got)
	}
}

func TestTheIdempotencyOutcomesOfTheContextReachTheClient(t *testing.T) {
	status := func(err error) error {
		mapped, _ := kernelgrpc.IdempotencyStatus(err)
		return mapped
	}
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"reused key":    {status(ports.ErrIdempotencyMismatch), http.StatusUnprocessableEntity, "reused-idempotency-key"},
		"key in flight": {status(ports.ErrIdempotencyInFlight), http.StatusConflict, "in-flight-idempotency-key"},
		"missing key":   {kernelgrpc.KeyStatus(kernelgrpc.ReasonMissingIdempotencyKey), http.StatusBadRequest, "missing-idempotency-key"},
		"invalid key":   {kernelgrpc.KeyStatus(kernelgrpc.ReasonInvalidIdempotencyKey), http.StatusBadRequest, "invalid-idempotency-key"},
		"exists":        {status(ports.ErrAlreadyExists), http.StatusConflict, "already-exists"},
	} {
		t.Run(name, func(t *testing.T) {
			fake := (&fakeContexts{}).on("ReserveBooking", func(int) (any, error) { return nil, c.err })
			f := newFixture(t, fake)

			rec := f.post(t, "/bookings/booking", `{"bookingId":"b-1","resourceId":"room-1","quantity":2}`)

			requireRejection(t, rec, c.status, c.code)
		})
	}
}

func TestTheAccessLogCarriesTheClientAndTheDerivedKey(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.post(t, "/orders/o-1/items", `{"sku":"A","quantity":1}`)

	record := onlyAccessLog(t, f)
	if record["dmpf.idempotency_key"] != "k-1" || record["dmpf.idempotency_key.derived"] != derived("tester", "k-1") {
		t.Fatalf("access log = %v, want the client key and the key the context received", record)
	}
}

func TestTheAccessLogFlagsAnInvalidKeyWithoutRecordingIt(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, http.MethodPost, "/orders/o-1/items", strings.NewReader(`{"sku":"A","quantity":1}`),
		"Idempotency-Key", "k 1\nforged=true", "Content-Type", "application/json", "Authorization", testCredential)

	record := onlyAccessLog(t, f)
	if _, logged := record["dmpf.idempotency_key"]; logged || record["dmpf.idempotency_key.invalid"] != true {
		t.Fatalf("access log = %v, want dmpf.idempotency_key.invalid and no dmpf.idempotency_key: the header is client input", record)
	}
}

func TestOnlyAReplayedAnswerIsMarked(t *testing.T) {
	status := func(err error) error {
		mapped, _ := kernelgrpc.IdempotencyStatus(err)
		return mapped
	}
	for name, c := range map[string]struct {
		fake   *fakeContexts
		status int
	}{
		"key in flight": {(&fakeContexts{}).on("PlaceOrder", func(int) (any, error) { return nil, status(ports.ErrIdempotencyInFlight) }), http.StatusConflict},
		"reused key":    {(&fakeContexts{}).on("PlaceOrder", func(int) (any, error) { return nil, status(ports.ErrIdempotencyMismatch) }), http.StatusUnprocessableEntity},
		"replay the edge cannot answer": {(&fakeContexts{}).replay("PlaceOrder").on("PlaceOrder", func(int) (any, error) {
			return &ordersv1.PlaceOrderResponse{}, nil
		}), http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			rec := newFixture(t, c.fake).post(t, "/orders/o-1/place", "")

			if rec.Code != c.status {
				t.Fatalf("status = %d %s, want %d", rec.Code, rec.Body.String(), c.status)
			}
			if got := rec.Header().Get("Idempotent-Replayed"); got != "" {
				t.Fatalf("Idempotent-Replayed = %q on a %d, want none: only a replayed answer below 500 is marked", got, rec.Code)
			}
		})
	}
}

func TestCORSExposesTheReplayHeader(t *testing.T) {
	f := newFixture(t, &fakeContexts{}, withCORS("https://app.example"))

	rec := f.do(t, http.MethodGet, "/orders/o-1", nil, "Origin", "https://app.example")

	exposed := rec.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposed, "Idempotent-Replayed") || !strings.Contains(exposed, "X-Correlation-ID") {
		t.Fatalf("Access-Control-Expose-Headers = %q, want X-Correlation-ID and Idempotent-Replayed", exposed)
	}
}
