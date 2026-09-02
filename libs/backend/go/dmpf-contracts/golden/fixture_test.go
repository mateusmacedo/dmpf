package golden

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/types/known/timestamppb"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
)

// The fixture lives in the contracts tree (FIX-10), five directories above this package.
const fixturePath = "../../../../../contracts/fixtures/orders/event/v1/order-placed.golden"

const (
	formatVersion = "1"
	fixtureID     = "orders/event/v1/order-placed"
	contractPkg   = "company.orders.event.v1"
	contractMsg   = "OrderPlaced"
	envelopeType  = "com.company.orders.order-placed.v1"
	dataSchema    = "type.googleapis.com/company.orders.event.v1.OrderPlaced"
)

// Every scalar is a JSON string (FIX-07): int64 and timestamps must survive a
// TypeScript reader without going through a double.
type fixtureDoc struct {
	FormatVersion  string        `json:"format_version"`
	Identity       identity      `json:"identity"`
	Covers         covers        `json:"covers"`
	Cases          []fixtureCase `json:"cases"`
	Discriminators []fixtureCase `json:"discriminators"`
}

type identity struct {
	Fixture    string   `json:"fixture"`
	Contract   contract `json:"contract"`
	Type       string   `json:"type"`
	DataSchema string   `json:"dataschema"`
}

type contract struct {
	Package string `json:"package"`
	Message string `json:"message"`
}

type covers struct {
	ProfileMajor  string `json:"profile_major"`
	ContractMajor string `json:"contract_major"`
}

type fixtureCase struct {
	Name            string            `json:"name"`
	Doc             string            `json:"doc"`
	Envelope        map[string]string `json:"envelope"`
	Payload         map[string]string `json:"payload"`
	PayloadBytesHex string            `json:"payload_bytes_hex"`
	PayloadHash     string            `json:"payload_hash"`
}

func baseEnvelope() map[string]string {
	return map[string]string{
		"id":              "evt-0001",
		"source":          "urn:lidercap:orders",
		"specversion":     envelope.SpecVersion,
		"type":            envelopeType,
		"subject":         "order/o-1001",
		"time":            "2026-09-02T12:00:00Z",
		"dataschema":      dataSchema,
		"datacontenttype": envelope.ContentType,
		"correlationid":   "corr-0001",
		"causationid":     "evt-0001",
		"partitionkey":    "c-42",
		"traceparent":     "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	}
}

func withConditionals(env map[string]string) map[string]string {
	env["aggregateversion"] = "7"
	env["tenantid"] = "tenant-a"
	env["tracestate"] = "vendor=1"
	return env
}

func payloadFields(orderID, customerID, totalCents, channel string) map[string]string {
	return map[string]string{
		"order_id":    orderID,
		"customer_id": customerID,
		"total_cents": totalCents,
		"channel":     channel,
	}
}

// messageFromFields is the single reader of the string-typed payload; the
// generator and the oracles share it so the two never disagree on parsing.
func messageFromFields(t *testing.T, fields map[string]string) *eventv1.OrderPlaced {
	t.Helper()
	total, err := strconv.ParseInt(fields["total_cents"], 10, 64)
	if err != nil {
		t.Fatalf("total_cents %q: %v", fields["total_cents"], err)
	}
	return &eventv1.OrderPlaced{
		OrderId:    fields["order_id"],
		CustomerId: fields["customer_id"],
		TotalCents: total,
		Channel:    channelFromString(t, fields["channel"]),
	}
}

func channelFromString(t *testing.T, s string) eventv1.OrderChannel {
	t.Helper()
	if v, ok := eventv1.OrderChannel_value[s]; ok {
		return eventv1.OrderChannel(v)
	}
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		t.Fatalf("channel %q is neither an enum name nor a number", s)
	}
	return eventv1.OrderChannel(n)
}

func envelopeFromFields(t *testing.T, fields map[string]string, payload []byte) envelope.Envelope {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, fields["time"])
	if err != nil {
		t.Fatalf("time %q: %v", fields["time"], err)
	}
	e := envelope.Envelope{
		ID:              fields["id"],
		Source:          fields["source"],
		SpecVersion:     fields["specversion"],
		Type:            fields["type"],
		Subject:         fields["subject"],
		Time:            timestamppb.New(ts),
		DataSchema:      fields["dataschema"],
		DataContentType: fields["datacontenttype"],
		CorrelationID:   fields["correlationid"],
		CausationID:     fields["causationid"],
		PartitionKey:    fields["partitionkey"],
		TraceParent:     fields["traceparent"],
		Payload:         payload,
	}
	if v, ok := fields["aggregateversion"]; ok {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			t.Fatalf("aggregateversion %q: %v", v, err)
		}
		av := int32(n)
		e.AggregateVersion = &av
	}
	if v, ok := fields["tenantid"]; ok {
		tenant := v
		e.TenantID = &tenant
	}
	if v, ok := fields["tracestate"]; ok {
		state := v
		e.TraceState = &state
	}
	return e
}

func packedCase(t *testing.T, name, doc string, env, fields map[string]string) fixtureCase {
	t.Helper()
	payload, typeURL, err := envelope.Pack(messageFromFields(t, fields))
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if typeURL != dataSchema {
		t.Fatalf("Pack type URL = %q, want %q", typeURL, dataSchema)
	}
	return rawCase(name, doc, env, fields, payload)
}

func rawCase(name, doc string, env, fields map[string]string, payload []byte) fixtureCase {
	return fixtureCase{
		Name:            name,
		Doc:             doc,
		Envelope:        env,
		Payload:         fields,
		PayloadBytesHex: hex.EncodeToString(payload),
		PayloadHash:     payloadhash.Sum(payload),
	}
}

// Field numbers of company.orders.event.v1.OrderPlaced, as declared in the .proto.
const (
	fieldOrderID    = 1
	fieldCustomerID = 2
	fieldTotalCents = 3
	fieldChannel    = 4
	fieldUnknown    = 7
)

func appendString(b []byte, num protowire.Number, v string) []byte {
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendString(b, v)
}

func appendVarint(b []byte, num protowire.Number, v uint64) []byte {
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

// Fields emitted in descending number order: valid wire, same decoded message,
// but never what a Go reserialization produces — the ENV-18 discriminator.
func nonCanonicalPayload(t *testing.T, fields map[string]string) []byte {
	t.Helper()
	msg := messageFromFields(t, fields)
	var b []byte
	b = appendVarint(b, fieldChannel, uint64(msg.GetChannel()))
	b = appendVarint(b, fieldTotalCents, uint64(msg.GetTotalCents()))
	b = appendString(b, fieldCustomerID, msg.GetCustomerId())
	b = appendString(b, fieldOrderID, msg.GetOrderId())
	return b
}

func buildFixture(t *testing.T) fixtureDoc {
	t.Helper()

	allPresent := packedCase(t, "all-conditionals-present",
		"Caminho típico com os três atributos condicionais presentes (aggregateversion, tenantid, tracestate).",
		withConditionals(baseEnvelope()), payloadFields("o-1001", "c-42", "1999", "ORDER_CHANNEL_WEB"))

	absentEnv := baseEnvelope()
	absentEnv["id"] = "evt-0002"
	absentEnv["causationid"] = "evt-0001"
	allAbsent := packedCase(t, "all-conditionals-absent",
		"Os três condicionais ausentes: nenhum entra no mapa de atributos (ENV-12, sem valor de preenchimento).",
		absentEnv, payloadFields("o-1002", "c-42", "250", "ORDER_CHANNEL_APP"))

	unspecEnv := baseEnvelope()
	unspecEnv["id"] = "evt-0003"
	unspecified := packedCase(t, "channel-unspecified",
		"Enum no valor zero (ORDER_CHANNEL_UNSPECIFIED, PTB-09): o campo não aparece no wire.",
		unspecEnv, payloadFields("o-1003", "c-7", "0", "ORDER_CHANNEL_UNSPECIFIED"))

	bigEnv := withConditionals(baseEnvelope())
	bigEnv["id"] = "evt-0004"
	bigEnv["aggregateversion"] = "2147483647"
	beyondDouble := packedCase(t, "total-cents-beyond-double",
		"total_cents = 2^53 + 1: um leitor que passar por double perde o último dígito (FIX-07).",
		bigEnv, payloadFields("o-1004", "c-42", "9007199254740993", "ORDER_CHANNEL_WEB"))

	nanosEnv := baseEnvelope()
	nanosEnv["id"] = "evt-0005"
	nanosEnv["time"] = "2026-09-02T12:00:00.123456789Z"
	nanosEnv["tracestate"] = "vendor=1,other=2"
	withNanos := packedCase(t, "time-with-nanos",
		"Instante do fato com nanossegundos não nulos; tracestate presente sem os outros condicionais.",
		nanosEnv, payloadFields("o-1005", "c-9", "12345", "ORDER_CHANNEL_APP"))

	unknownFields := payloadFields("o-2001", "c-42", "1999", "ORDER_CHANNEL_WEB")
	canonical, _, err := envelope.Pack(messageFromFields(t, unknownFields))
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	unknownEnv := baseEnvelope()
	unknownEnv["id"] = "evt-2001"
	unknownField := rawCase("unknown-field",
		"Bytes canônicos mais um campo de número 7 (varint 42) que o contrato não conhece: a desserialização o preserva (PTB-10) e o hash o cobre (ENV-17).",
		unknownEnv, unknownFields, appendVarint(append([]byte(nil), canonical...), fieldUnknown, 42))

	unknownEnumEnv := baseEnvelope()
	unknownEnumEnv["id"] = "evt-2002"
	unknownEnum := packedCase(t, "enum-unknown-value",
		"channel = 99, valor que o enum não declara: decodifica sem erro e é preservado numericamente.",
		unknownEnumEnv, payloadFields("o-2002", "c-42", "1999", "99"))

	nonCanonicalFields := payloadFields("o-2003", "c-42", "1999", "ORDER_CHANNEL_WEB")
	nonCanonicalEnv := baseEnvelope()
	nonCanonicalEnv["id"] = "evt-2003"
	nonCanonical := rawCase("non-canonical-field-order",
		"Campos em ordem decrescente de número: decodifica no mesmo valor, mas o hash é o dos bytes transportados e difere do hash de uma reserialização (ENV-18).",
		nonCanonicalEnv, nonCanonicalFields, nonCanonicalPayload(t, nonCanonicalFields))

	return fixtureDoc{
		FormatVersion: formatVersion,
		Identity: identity{
			Fixture:    fixtureID,
			Contract:   contract{Package: contractPkg, Message: contractMsg},
			Type:       envelopeType,
			DataSchema: dataSchema,
		},
		Covers:         covers{ProfileMajor: "1", ContractMajor: "v1"},
		Cases:          []fixtureCase{allPresent, allAbsent, unspecified, beyondDouble, withNanos},
		Discriminators: []fixtureCase{unknownField, unknownEnum, nonCanonical},
	}
}

// TestUpdateGolden rewrites the fixture from code; it only runs with GOLDEN_UPDATE=1
// so the committed file stays the reviewed oracle, never a side effect of `go test`.
func TestUpdateGolden(t *testing.T) {
	if os.Getenv("GOLDEN_UPDATE") != "1" {
		t.Skip("set GOLDEN_UPDATE=1 to regenerate the golden fixture")
	}
	doc := buildFixture(t)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(fixturePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixturePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("golden fixture written to %s (%d bytes)", fixturePath, len(data))
}
