package golden_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"google.golang.org/protobuf/proto"

	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/golden"
)

var itemAdded = golden.Subject{
	NewMessage: func() proto.Message { return &eventv1.ItemAdded{} },
	MessageFromFields: func(fields map[string]string) (proto.Message, error) {
		q, err := strconv.ParseInt(fields["quantity"], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("quantity %q: %w", fields["quantity"], err)
		}
		return &eventv1.ItemAdded{OrderId: fields["order_id"], Sku: fields["sku"], Quantity: int32(q)}, nil
	},
}

var orderPlaced = golden.Subject{
	NewMessage: func() proto.Message { return &eventv1.OrderPlaced{} },
	MessageFromFields: func(fields map[string]string) (proto.Message, error) {
		total, err := strconv.ParseInt(fields["total_cents"], 10, 64)
		if err != nil {
			return nil, err
		}
		return &eventv1.OrderPlaced{
			OrderId: fields["order_id"], CustomerId: fields["customer_id"], TotalCents: total,
			Channel: eventv1.OrderChannel(eventv1.OrderChannel_value[fields["channel"]]),
		}, nil
	},
}

func conformingCase(t *testing.T) (golden.Fixture, golden.Case) {
	t.Helper()
	f := readItemAdded(t)
	c, ok := f.FindCase("all-conditionals-present")
	if !ok {
		t.Fatal("case all-conditionals-present missing")
	}
	return f, c
}

func TestAConformingCaseYieldsSixPasses(t *testing.T) {
	f, c := conformingCase(t)
	var rep golden.Report
	rep.Add(golden.Consumer{}.Run(f, c, itemAdded)...)
	rep.Add(golden.Producer{}.Run(f, c, itemAdded)...)
	if len(rep.Outcomes) != 6 {
		t.Fatalf("%d outcomes, want 6", len(rep.Outcomes))
	}
	if failed := rep.Failed(); len(failed) != 0 {
		t.Fatalf("failures on a conforming case: %+v", failed)
	}
}

// Bytes altered in one field, hash recomputed over the altered bytes: the
// consumer's oracle 2 passes and oracle 1 names the field; the producer fails
// oracle 3 and oracle 2 as two distinct outcomes.
func TestEachOracleFailsOnItsOwn(t *testing.T) {
	f, c := conformingCase(t)
	altered := decodeHex(t, c.PayloadBytesHex)
	altered[len(altered)-1] = 3 // quantity 2 → 3 on the wire
	c.PayloadBytesHex = hex.EncodeToString(altered)
	c.PayloadHash = payloadhash.Sum(altered)

	consumer := byOracle(golden.Consumer{}.Run(f, c, itemAdded))
	if !consumer[golden.OracleHash].OK {
		t.Errorf("consumer oracle 2 failed although the hash covers the altered bytes: %+v", consumer[golden.OracleHash])
	}
	if o := consumer[golden.OracleSemantic]; o.OK || o.Code != golden.CodeR001 || o.Field != "quantity" || o.Expected != "2" || o.Got != "3" {
		t.Errorf("consumer oracle 1 = %+v, want R001 on quantity 2 → 3", o)
	}
	if !consumer[golden.OracleBytes].OK {
		t.Errorf("consumer oracle 3 failed: Any.value must survive the envelope trip regardless of content")
	}

	producer := byOracle(golden.Producer{}.Run(f, c, itemAdded))
	if o := producer[golden.OracleBytes]; o.OK || o.Code != golden.CodeR003 || o.Field != "payload_bytes_hex" {
		t.Errorf("producer oracle 3 = %+v, want R003", o)
	}
	if o := producer[golden.OracleHash]; o.OK || o.Code != golden.CodeR002 {
		t.Errorf("producer oracle 2 = %+v, want R002", o)
	}
	if !producer[golden.OracleSemantic].OK {
		t.Errorf("producer oracle 1 failed: the declared envelope still encodes")
	}
}

func TestStaleHashFailsOracleTwoOnly(t *testing.T) {
	f, c := conformingCase(t)
	c.PayloadHash = payloadhash.Sum([]byte("something else"))
	consumer := byOracle(golden.Consumer{}.Run(f, c, itemAdded))
	if o := consumer[golden.OracleHash]; o.OK || o.Code != golden.CodeR002 || o.Field != "payload_hash" {
		t.Fatalf("oracle 2 = %+v, want R002 on payload_hash", o)
	}
	if !consumer[golden.OracleSemantic].OK || !consumer[golden.OracleBytes].OK {
		t.Fatalf("a stale hash must not fail the other oracles: %+v", consumer)
	}
}

func TestEnvelopeAttributeThatDoesNotSurviveIsNamed(t *testing.T) {
	f, c := conformingCase(t)
	c.Envelope["time"] = "not-a-timestamp"
	consumer := byOracle(golden.Consumer{}.Run(f, c, itemAdded))
	if o := consumer[golden.OracleSemantic]; o.OK || o.Field != "envelope.time" {
		t.Fatalf("oracle 1 = %+v, want a failure on envelope.time", o)
	}
}

func TestInt64BeyondDoubleSurvivesBothDirections(t *testing.T) {
	const beyond = int64(9007199254740993)
	msg := &eventv1.OrderPlaced{OrderId: "o-1", CustomerId: "c-1", TotalCents: beyond, Channel: eventv1.OrderChannel_ORDER_CHANNEL_WEB}
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	f := golden.Fixture{FormatVersion: "1", Identity: golden.Identity{Fixture: "synthetic/order-placed", DataSchema: "type.googleapis.com/company.orders.event.v1.OrderPlaced", Type: "com.company.orders.order-placed.v1"}}
	_, base := conformingCase(t)
	env := map[string]string{}
	for k, v := range base.Envelope {
		env[k] = v
	}
	env["type"], env["dataschema"] = f.Identity.Type, f.Identity.DataSchema
	c := golden.Case{
		Name: "beyond-double", Envelope: env,
		Payload:         map[string]string{"order_id": "o-1", "customer_id": "c-1", "total_cents": strconv.FormatInt(beyond, 10), "channel": "ORDER_CHANNEL_WEB"},
		PayloadBytesHex: hex.EncodeToString(wire), PayloadHash: payloadhash.Sum(wire),
	}
	f.Cases = []golden.Case{c}
	rep := golden.Evaluate(f, orderPlaced)
	if failed := rep.Failed(); len(failed) != 0 {
		t.Fatalf("int64 beyond 2^53 did not survive: %+v", failed)
	}
	if len(rep.Outcomes) != 6 {
		t.Fatalf("%d outcomes, want 6", len(rep.Outcomes))
	}
}

func TestEvaluateRunsProducerOnlyOverCanonicalCases(t *testing.T) {
	f := readItemAdded(t)
	rep := golden.Evaluate(f, itemAdded)
	if want := 3*len(f.AllCases()) + 3*len(f.Cases); len(rep.Outcomes) != want {
		t.Fatalf("%d outcomes, want %d", len(rep.Outcomes), want)
	}
	if failed := rep.Failed(); len(failed) != 0 {
		t.Fatalf("the committed fixture does not round trip: %+v", failed)
	}
	for _, o := range rep.Outcomes {
		if o.Direction == golden.DirectionProducer {
			if _, isDiscriminator := indexOf(f.Discriminators, o.Case); isDiscriminator {
				t.Fatalf("producer ran over discriminator %s", o.Case)
			}
		}
	}
}

func TestReportSerializesTheSameRegardlessOfInsertionOrder(t *testing.T) {
	f, c := conformingCase(t)
	c.PayloadHash = payloadhash.Sum([]byte("stale"))
	outcomes := append(golden.Consumer{}.Run(f, c, itemAdded), golden.Producer{}.Run(f, c, itemAdded)...)
	var forward, backward golden.Report
	forward.Add(outcomes...)
	for i := len(outcomes) - 1; i >= 0; i-- {
		backward.Add(outcomes[i])
	}
	a, err := json.Marshal(forward)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(backward)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("report JSON depends on insertion order:\n%s\n%s", a, b)
	}
	if !bytes.Contains(a, []byte(`"code":"DMPF-R002"`)) || !bytes.Contains(a, []byte(`"ok":false`)) {
		t.Fatalf("report does not carry the failure: %s", a)
	}
}

func byOracle(outcomes []golden.Outcome) map[golden.Oracle]golden.Outcome {
	out := map[golden.Oracle]golden.Outcome{}
	for _, o := range outcomes {
		out[o.Oracle] = o
	}
	return out
}

func indexOf(cases []golden.Case, name string) (int, bool) {
	for i, c := range cases {
		if c.Name == name {
			return i, true
		}
	}
	return -1, false
}

func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
