//go:build integration

package bff_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
)

const (
	orderPlacedType          = "com.company.orders.order-placed.v1"
	reservationConfirmedType = "com.company.reservations.reservation-confirmed.v1"
	reservationCancelledType = "com.company.reservations.reservation-cancelled.v1"

	correlation = "corr-e2e-1"
	traceID     = "4bf92f3577b34da6a3ce929d0e0e4736"
	traceparent = "00-" + traceID + "-00f067aa0ba902b7-01"

	reservedOrder = "o-e2e-reserved"
	canceledOrder = "o-e2e-canceled"
)

func TestTopologyEndToEnd(t *testing.T) {
	top := newTopology(t)
	bin := buildBinaries(t)
	top.boot(t, bin)

	t.Log("1. cancel a reservation before its order is placed")
	if status, body := top.call(t, http.MethodPost, "/reservations/"+canceledOrder+"/cancel", "", nil); status != http.StatusOK {
		t.Fatalf("POST cancel: status = %d, body %s", status, body)
	}

	t.Log("2. place two orders through the BFF, the first with a known correlation and trace")
	headers := map[string]string{"X-Correlation-ID": correlation, "traceparent": traceparent}
	top.placeOrder(t, reservedOrder, headers)
	top.placeOrder(t, canceledOrder, nil)

	placed := top.envelopeOf(t, top.ordersTopic, orderPlacedType, reservedOrder)
	if placed.CorrelationID != correlation {
		t.Fatalf("OrderPlaced correlationid = %q, want %q (CTX-07)", placed.CorrelationID, correlation)
	}
	if cause := placed.CausationID; cause == "" || cause == placed.ID {
		t.Fatalf("OrderPlaced causationid = %q, want the request id of the context (CTX-08)", cause)
	}
	if got := traceOf(placed.TraceParent); got != traceID {
		t.Fatalf("OrderPlaced trace id = %q, want %q from the BFF edge", got, traceID)
	}

	t.Log("3. the reservation of the first order is confirmed and its fact leaves with the chain intact")
	top.waitUntil(t, "the first reservation is confirmed", func() bool {
		return top.reservationStatus(t, reservedOrder) == "confirmed"
	})
	confirmed := top.envelopeOf(t, top.reservationsTopic, reservationConfirmedType, reservedOrder)
	if confirmed.CorrelationID != correlation {
		t.Fatalf("ReservationConfirmed correlationid = %q, want %q", confirmed.CorrelationID, correlation)
	}
	if confirmed.CausationID != placed.ID {
		t.Fatalf("ReservationConfirmed causationid = %q, want the OrderPlaced id %q", confirmed.CausationID, placed.ID)
	}
	if got := traceOf(confirmed.TraceParent); got != traceID {
		t.Fatalf("ReservationConfirmed trace id = %q, want %q", got, traceID)
	}

	t.Log("4. the canceled reservation refuses the later OrderPlaced: first decision wins")
	top.envelopeOf(t, top.ordersTopic, orderPlacedType, canceledOrder)
	top.waitUntil(t, "the group consumed the OrderPlaced of the canceled order", func() bool {
		return top.consumedEverything(t, top.ordersTopic)
	})
	if status := top.reservationStatus(t, canceledOrder); status != "canceled" {
		t.Fatalf("canceled reservation status = %q, want canceled", status)
	}
	top.envelopeOf(t, top.reservationsTopic, reservationCancelledType, canceledOrder)
	if n := top.countOnTopic(t, top.reservationsTopic, reservationConfirmedType, canceledOrder); n != 0 {
		t.Fatalf("ReservationConfirmed facts for the canceled order = %d, want 0", n)
	}

	t.Log("5. redeliver the first OrderPlaced: nothing changes")
	top.republish(t, top.ordersTopic, orderPlacedType, reservedOrder)
	top.waitUntil(t, "the group consumed the redelivery", func() bool { return top.consumedEverything(t, top.ordersTopic) })
	if n := top.countOnTopic(t, top.reservationsTopic, reservationConfirmedType, reservedOrder); n != 1 {
		t.Fatalf("ReservationConfirmed facts for the redelivered order = %d, want 1 (DuplicateIgnored)", n)
	}
	for _, order := range []string{reservedOrder, canceledOrder} {
		if status := top.reservationStatus(t, order); status == "" {
			t.Fatalf("reservation of %s is not readable through the BFF", order)
		}
	}
	if n := len(top.recordsUntilEnd(t, top.ordersDLQ)); n != 0 {
		t.Fatalf("records contained on the DLQ of the consumed topic = %d, want 0", n)
	}

	t.Log("6. each context publishes only its own facts")
	requireTypePrefix(t, top.recordsUntilEnd(t, top.ordersTopic), "com.company.orders.")
	requireTypePrefix(t, top.recordsUntilEnd(t, top.reservationsTopic), "com.company.reservations.")

	t.Log("7. the same call without a credential is refused as identity, not as an internal failure")
	if status, body := top.callAnonymous(t, http.MethodGet, "/orders/"+reservedOrder); status != http.StatusUnauthorized {
		t.Fatalf("GET order without credential: status = %d, want %d (IDN-01, IDN-06), body %s", status, http.StatusUnauthorized, body)
	}
}

func (top *topology) placeOrder(t *testing.T, order string, headers map[string]string) {
	t.Helper()
	if status, body := top.call(t, http.MethodPost, "/orders/"+order+"/items", `{"sku":"A","quantity":1}`, headers); status != http.StatusCreated {
		t.Fatalf("POST items %s: status = %d, body %s", order, status, body)
	}
	if status, body := top.call(t, http.MethodPost, "/orders/"+order+"/place", "", headers); status != http.StatusOK {
		t.Fatalf("POST place %s: status = %d, body %s", order, status, body)
	}
}

// e2eCredential is what the development authenticator reads back. The tenant
// matches the one admission declares, so the identity the edge resolves and the
// bucket it charges stay the same until Phase 7 removes the literal.
const e2eCredential = `Bearer {"sub":"e2e-tester","tenant":"acme","permissions":["orders:write","orders:read","reservations:write","reservations:read"]}`

func (top *topology) call(t *testing.T, method, path, body string, headers map[string]string) (int, []byte) {
	t.Helper()
	req := top.request(t, method, path, body)
	req.Header.Set("Authorization", e2eCredential)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return send(t, req)
}

// callAnonymous sends the same request without the Authorization header, which
// is the only way to observe the refusal across the whole topology.
func (top *topology) callAnonymous(t *testing.T, method, path string) (int, []byte) {
	t.Helper()
	return send(t, top.request(t, method, path, ""))
}

func (top *topology) request(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+top.bffAddr+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest() = %v", err)
	}
	if method == http.MethodPost {
		req.Header.Set("Idempotency-Key", fmt.Sprintf("k-%d", time.Now().UnixNano()))
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func send(t *testing.T, req *http.Request) (int, []byte) {
	t.Helper()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer res.Body.Close()
	got, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("ReadAll() = %v", err)
	}
	return res.StatusCode, got
}

func (top *topology) reservationStatus(t *testing.T, order string) string {
	t.Helper()
	status, body := top.call(t, http.MethodGet, "/reservations/"+order, "", nil)
	if status != http.StatusOK {
		return ""
	}
	var view struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("GET reservation body %s: %v", body, err)
	}
	return view.Status
}

func requireTypePrefix(t *testing.T, envelopes []envelope.Envelope, prefix string) {
	t.Helper()
	if len(envelopes) == 0 {
		t.Fatalf("no fact published; want facts of type %s*", prefix)
	}
	for _, env := range envelopes {
		if !strings.HasPrefix(env.Type, prefix) {
			t.Fatalf("fact of type %q on a topic that should carry only %s*", env.Type, prefix)
		}
	}
}

func traceOf(parent string) string {
	parts := strings.Split(parent, "-")
	if len(parts) != 4 {
		return ""
	}
	return parts[1]
}

func (top *topology) envelopeOf(t *testing.T, topic, eventType, aggregate string) envelope.Envelope {
	t.Helper()
	env, _ := top.findRecord(t, topic, eventType, aggregate)
	return env
}

func (top *topology) findRecord(t *testing.T, topic, eventType, aggregate string) (envelope.Envelope, *kgo.Record) {
	t.Helper()
	cl, err := kgo.NewClient(kgo.SeedBrokers(top.brokers...), kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatalf("kgo.NewClient() = %v", err)
	}
	defer cl.Close()
	deadline := time.Now().Add(e2eTimeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		fetches := cl.PollFetches(ctx)
		cancel()
		var (
			found  envelope.Envelope
			record *kgo.Record
		)
		fetches.EachRecord(func(r *kgo.Record) {
			if record != nil {
				return
			}
			if env, err := envelope.Unmarshal(r.Value); err == nil && env.Type == eventType && env.PartitionKey == aggregate {
				found, record = env, r
			}
		})
		if record != nil {
			return found, record
		}
	}
	t.Fatalf("no %s for %s arrived on %s within %v", eventType, aggregate, topic, e2eTimeout)
	return envelope.Envelope{}, nil
}

func (top *topology) countOnTopic(t *testing.T, topic, eventType, aggregate string) int {
	t.Helper()
	n := 0
	for _, env := range top.recordsUntilEnd(t, topic) {
		if env.Type == eventType && env.PartitionKey == aggregate {
			n++
		}
	}
	return n
}

// recordsUntilEnd drains the topic up to the end offsets read before the first
// fetch: an absence is proven over everything published so far, not over
// whatever one poll happened to return.
func (top *topology) recordsUntilEnd(t *testing.T, topic string) []envelope.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), adminWindow)
	defer cancel()
	ends, err := top.admin.ListEndOffsets(ctx, topic)
	if err != nil {
		t.Fatalf("ListEndOffsets(%s) = %v", topic, err)
	}
	remaining := map[int32]int64{}
	ends.Each(func(o kadm.ListedOffset) {
		if o.Offset > 0 {
			remaining[o.Partition] = o.Offset
		}
	})
	if len(remaining) == 0 {
		return nil
	}
	cl, err := kgo.NewClient(kgo.SeedBrokers(top.brokers...), kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatalf("kgo.NewClient() = %v", err)
	}
	defer cl.Close()
	var envelopes []envelope.Envelope
	deadline := time.Now().Add(e2eTimeout)
	for len(remaining) > 0 && time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		fetches := cl.PollFetches(ctx)
		cancel()
		fetches.EachRecord(func(r *kgo.Record) {
			end, open := remaining[r.Partition]
			if !open {
				return
			}
			if env, err := envelope.Unmarshal(r.Value); err == nil {
				envelopes = append(envelopes, env)
			} else {
				envelopes = append(envelopes, envelope.Envelope{Type: "<unreadable>"})
			}
			if r.Offset+1 >= end {
				delete(remaining, r.Partition)
			}
		})
	}
	if len(remaining) > 0 {
		t.Fatalf("%s was not read to its end offsets within %v", topic, e2eTimeout)
	}
	return envelopes
}

func (top *topology) republish(t *testing.T, topic, eventType, aggregate string) {
	t.Helper()
	_, original := top.findRecord(t, topic, eventType, aggregate)
	cl, err := kgo.NewClient(kgo.SeedBrokers(top.brokers...))
	if err != nil {
		t.Fatalf("kgo.NewClient() = %v", err)
	}
	defer cl.Close()
	ctx, cancel := context.WithTimeout(context.Background(), adminWindow)
	defer cancel()
	copy := &kgo.Record{Topic: topic, Key: original.Key, Value: original.Value, Headers: original.Headers}
	if err := cl.ProduceSync(ctx, copy).FirstErr(); err != nil {
		t.Fatalf("ProduceSync() = %v", err)
	}
}

func (top *topology) consumedEverything(t *testing.T, topic string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), adminWindow)
	defer cancel()
	ends, err := top.admin.ListEndOffsets(ctx, topic)
	if err != nil {
		return false
	}
	var end int64
	ends.Each(func(o kadm.ListedOffset) { end += o.Offset })
	committed, err := top.admin.FetchOffsets(ctx, top.group)
	if err != nil {
		return false
	}
	var at int64
	committed.Each(func(o kadm.OffsetResponse) {
		if o.Topic == topic && o.At > 0 {
			at += o.At
		}
	})
	return end > 0 && at >= end
}
