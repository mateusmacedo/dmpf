package s3

import (
	"context"
	"strings"
	"sync"
	"testing"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type recordingExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range records {
		e.records = append(e.records, r.Clone())
	}
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error   { return nil }
func (e *recordingExporter) ForceFlush(context.Context) error { return nil }

// TestTelemetryCarriesNeitherKeyNorBody runs a write and a refused read and
// proves that no span, event nor log record carries the physical key, the body
// or the text of the S3 answer: the failure leaves only its category (DAT-22).
func TestTelemetryCarriesNeitherKeyNorBody(t *testing.T) {
	const secretKey, secretBody = "secret-key-42", "secret-body-42"
	recorder := tracetest.NewSpanRecorder()
	exporter := &recordingExporter{}
	cfg := unitConfig()
	cfg.Tracer = sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("s3-test")
	cfg.LoggerProvider = sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	api := &fakeAPI{
		putObject: putOK,
		headObject: func(in *awss3.HeadObjectInput) (*awss3.HeadObjectOutput, error) {
			return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied on " + *in.Key + " " + secretBody, Fault: smithy.FaultClient}
		},
	}
	store := readyStore(t, cfg, api)
	ctx := tenantCtx(t, "acme")
	body := []byte(secretBody)
	digest, _ := digestOf(body)
	if _, err := store.Put(ctx, ports.PutRequest{Key: secretKey, Body: body, ContentSHA256: digest}); err != nil {
		t.Fatalf("Put() = %v", err)
	}
	if _, err := store.Head(ctx, ports.ObjectRef{Key: secretKey, Version: "v1"}); err == nil {
		t.Fatal("Head() = nil, want the refusal")
	}

	leaks := func(where, text string) {
		if strings.Contains(text, secretKey) || strings.Contains(text, secretBody) {
			t.Errorf("%s carries the key or the body: %q", where, text)
		}
	}
	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("no span ended: the composition did not trace the calls")
	}
	for _, span := range spans {
		leaks("span name", span.Name())
		leaks("span status", span.Status().Description)
		for _, kv := range span.Attributes() {
			leaks("span attribute "+string(kv.Key), kv.Value.String())
		}
		for _, event := range span.Events() {
			leaks("span event", event.Name)
			for _, kv := range event.Attributes {
				leaks("event attribute "+string(kv.Key), kv.Value.String())
			}
		}
	}
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	for _, record := range exporter.records {
		leaks("log body", record.Body().String())
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			leaks("log attribute "+string(kv.Key), kv.Value.String())
			return true
		})
	}
}
