package sqs_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	awslog "github.com/aws/smithy-go/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
)

const (
	awsScope        = "github.com/aws/aws-sdk-go-v2"
	awsAccessKey    = "AKIDLEAKEDACCESSKEY"
	awsSessionToken = "LEAKED-SESSION-TOKEN"
	awsPayload      = "payload of a customer 123.456.789-00"
	awsBadDate      = "not-a-date-from-10.0.0.7"
)

type awsRecord struct {
	scope, level, body string
	traceID            trace.TraceID
	values             []string
}

type awsRecords struct {
	mu      sync.Mutex
	records []awsRecord
}

func (l *awsRecords) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }
func (l *awsRecords) Shutdown(context.Context) error                         { return nil }
func (l *awsRecords) ForceFlush(context.Context) error                       { return nil }

func (l *awsRecords) OnEmit(_ context.Context, record *sdklog.Record) error {
	got := awsRecord{
		scope:   record.InstrumentationScope().Name,
		level:   record.SeverityText(),
		body:    record.Body().AsString(),
		traceID: record.TraceID(),
	}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		got.values = append(got.values, string(kv.Key)+"="+kv.Value.String())
		return true
	})
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, got)
	return nil
}

func (l *awsRecords) under(scope string) []awsRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []awsRecord
	for _, record := range l.records {
		if record.scope == scope {
			out = append(out, record)
		}
	}
	return out
}

func awsServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", awsBadDate)
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-amz-json") {
			w.Header().Set("Content-Type", "application/x-amz-json-1.0")
			_, _ = io.WriteString(w, `{"MessageId":"m-1"}`)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = io.WriteString(w, `<PublishResponse><PublishResult><MessageId>m-1</MessageId></PublishResult></PublishResponse>`)
	}))
	t.Cleanup(server.Close)
	return server
}

func stubbedAWSConfig(server *httptest.Server, stderr io.Writer, logs log.LoggerProvider) sqs.Config {
	cfg := validConfig()
	cfg.Endpoint = server.URL
	cfg.LoggerProvider = logs
	cfg.AWS = aws.Config{
		Region:     "us-east-1",
		HTTPClient: server.Client(),
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: awsAccessKey, SecretAccessKey: "secret", SessionToken: awsSessionToken}, nil
		}),
		RetryMaxAttempts: 1,
		Logger:           awslog.NewStandardLogger(stderr),
		ClientLogMode:    aws.LogSigning | aws.LogRequestWithBody | aws.LogResponseWithBody,
	}
	return cfg
}

var awsCalls = map[string]func(context.Context, sqs.Config, string) error{
	"sqs": func(ctx context.Context, cfg sqs.Config, url string) error {
		_, err := sqs.NewSQSClient(cfg).SendMessage(ctx, &awssqs.SendMessageInput{
			QueueUrl: aws.String(url + "/000000000000/notifications"), MessageBody: aws.String(awsPayload),
		})
		return err
	},
	"sns": func(ctx context.Context, cfg sqs.Config, _ string) error {
		_, err := sqs.NewSNSClient(cfg).Publish(ctx, &awssns.PublishInput{
			TopicArn: aws.String(topicARN), Message: aws.String(awsPayload),
		})
		return err
	},
}

func TestTheSDKLogsThroughTheProviderUnderItsScopeAndNeverItsText(t *testing.T) {
	caller := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36},
		SpanID:  trace.SpanID{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7}, TraceFlags: trace.FlagsSampled,
	})
	for name, call := range awsCalls {
		t.Run(name, func(t *testing.T) {
			server := awsServer(t)
			var stderr bytes.Buffer
			records := &awsRecords{}
			cfg := stubbedAWSConfig(server, &stderr, sdklog.NewLoggerProvider(sdklog.WithProcessor(records)))

			if err := call(trace.ContextWithSpanContext(context.Background(), caller), cfg, server.URL); err != nil {
				t.Fatalf("call against the stub: %v", err)
			}

			if stderr.Len() != 0 {
				t.Errorf("the SDK wrote to stderr, the log only leaves by OTLP (RF-A1):\n%s", stderr.String())
			}
			got := records.under(awsScope)
			levels := map[string]bool{}
			for _, record := range got {
				levels[record.level] = true
				if record.body != got[0].body {
					t.Errorf("body %q differs from %q: the SDK text reached the record", record.body, got[0].body)
				}
				if record.traceID != caller.TraceID() {
					t.Errorf("%s record without the trace of the call: %s", record.level, record.traceID)
				}
				text := record.body + " " + strings.Join(record.values, " ")
				for _, leaked := range []string{awsAccessKey, awsSessionToken, awsPayload, awsBadDate} {
					if strings.Contains(text, leaked) {
						t.Errorf("%s record carries %q (DAT-02): %s", record.level, leaked, text)
					}
				}
			}
			if !levels["WARN"] || !levels["DEBUG"] {
				t.Fatalf("records under %s by level = %v, want the Date warning as WARN and the dumps as DEBUG", awsScope, levels)
			}
		})
	}
}

func TestWithoutAProviderTheSDKLogIsDiscardedAndNotWrittenToStderr(t *testing.T) {
	for name, call := range awsCalls {
		t.Run(name, func(t *testing.T) {
			server := awsServer(t)
			var stderr bytes.Buffer

			if err := call(context.Background(), stubbedAWSConfig(server, &stderr, nil), server.URL); err != nil {
				t.Fatalf("call against the stub: %v", err)
			}

			if stderr.Len() != 0 {
				t.Fatalf("without a LoggerProvider the SDK wrote to stderr (RF-A1):\n%s", stderr.String())
			}
		})
	}
}
