package http_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
)

type categorizedFailure struct{}

func (categorizedFailure) Error() string         { return "orders 10.0.0.7 refused" }
func (categorizedFailure) ErrorCategory() string { return "Unexpected" }

func TestEveryFailureIsRecordedUnderAnFND07CategoryOrOther(t *testing.T) {
	other := semconv.ErrorTypeOther.Value.AsString()
	status := func(code int) error { return &provider.RetryableStatusError{Status: code} }
	cases := map[string]struct {
		err  error
		want string
	}{
		"success":                  {nil, "ok"},
		"categorized":              {fmt.Errorf("wrapped: %w", categorizedFailure{}), "Unexpected"},
		"context deadline":         {fmt.Errorf("orders: secret: %w", context.DeadlineExceeded), "DeadlineExceeded"},
		"context cancelled":        {fmt.Errorf("orders: secret: %w", context.Canceled), "Cancelled"},
		"deadline in a url.Error":  {&url.Error{Op: "Get", URL: "http://orders", Err: context.DeadlineExceeded}, "DeadlineExceeded"},
		"cancelled in a url.Error": {&url.Error{Op: "Get", URL: "http://orders", Err: context.Canceled}, "Cancelled"},
		"network":                  {&url.Error{Op: "Get", URL: "http://orders", Err: &timeoutError{}}, "TransientDependency"},
		"plain error":              {errors.New("orders: secret"), other},
		"400":                      {status(http.StatusBadRequest), "Validation"},
		"401":                      {status(http.StatusUnauthorized), "Unauthenticated"},
		"403":                      {status(http.StatusForbidden), "Forbidden"},
		"404":                      {status(http.StatusNotFound), "NotFound"},
		"409":                      {status(http.StatusConflict), "Conflict"},
		"422":                      {status(http.StatusUnprocessableEntity), "DomainRejection"},
		"429":                      {status(http.StatusTooManyRequests), "RateLimited"},
		"500":                      {status(http.StatusInternalServerError), "TransientDependency"},
		"502":                      {status(http.StatusBadGateway), "TransientDependency"},
		"503":                      {fmt.Errorf("attempt: %w", status(http.StatusServiceUnavailable)), "TransientDependency"},
		"504":                      {status(http.StatusGatewayTimeout), "TransientDependency"},
		"599":                      {status(599), "TransientDependency"},
		"408":                      {status(http.StatusRequestTimeout), other},
		"410":                      {status(http.StatusGone), other},
		"499":                      {status(499), other},
		"600":                      {status(600), other},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := provider.CategoryOf(c.err); got != c.want {
				t.Fatalf("CategoryOf(%v) = %q, want %q: FND-07 read back from the status, the context or the network, or %s (RF-B1)", c.err, got, c.want, other)
			}
		})
	}
}

func TestATransientStatusIsRecordedAsTransientDependencyOnTheSpanAndTheLog(t *testing.T) {
	exporter := &recordingExporter{}
	up := newUpstream(t, http.StatusServiceUnavailable)
	spans, tracer := tracedConfig(t)
	cfg := clientConfig(route(http.MethodGet))
	cfg.Tracer = tracer
	cfg.LoggerProvider = sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	client := newClient(t, cfg)

	resp, err := client.Do(budgeted(t, 2*time.Second), "placeOrder", request(t, http.MethodGet, up.URL, ""))
	if err != nil {
		t.Fatalf("Do() = %v, want the last 503 response", err)
	}
	_ = resp.Body.Close()

	found := false
	for _, span := range spans() {
		if span.Name != "dmpf.resilience orders" {
			continue
		}
		found = true
		if got := attrsOf(span)["error.type"].AsString(); got != "TransientDependency" {
			t.Errorf("dmpf.resilience orders: error.type = %q, want TransientDependency, never the status (RF-B1)", got)
		}
	}
	if !found {
		t.Fatal("no dmpf.resilience orders span")
	}
	failed := 0
	for _, record := range exporter.snapshot() {
		if record.Body().AsString() != "transport: call failed" {
			continue
		}
		failed++
		errorType := ""
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			if kv.Key == semconv.ErrorTypeKey {
				errorType = kv.Value.AsString()
			}
			return true
		})
		if errorType != "TransientDependency" {
			t.Errorf("transport: call failed: error.type = %q, want TransientDependency (RF-B1)", errorType)
		}
	}
	if failed == 0 {
		t.Fatal("no failed attempt was logged")
	}
}
