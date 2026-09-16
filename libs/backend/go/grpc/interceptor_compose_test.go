package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
)

// composeConfig is a client whose sheet retries with a deterministic 100ms
// backoff and whose breaker never opens within a test.
func composeConfig(c *clock.Fake, idempotent bool, retryable ...codes.Code) provider.Config {
	cfg := validConfig()
	cfg.Clock = c
	cfg.Service = "checkout"
	cfg.Rand = func() float64 { return 0 }
	cfg.Sheet.Backoff = resilience.Declare(resilience.BackoffPolicy{Base: 100 * time.Millisecond, Factor: 2, Cap: time.Second})
	cfg.Sheet.MaxAttempts = resilience.Declare(3)
	p := policy(idempotent)
	p.RetryableCodes = retryable
	cfg.Methods = map[string]provider.MethodPolicy{checkMethod: p}
	return cfg
}

func budgeted(c *clock.Fake) (context.Context, context.CancelFunc) {
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	return retry.WithBudget(ctx, retry.WithTotal(10*time.Second)), cancel
}

// failing is an invoker that fails with the code n times before succeeding.
func failing(n int, code codes.Code, invoked *int) grpc.UnaryInvoker {
	return func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		*invoked++
		if *invoked <= n {
			return status.Error(code, "transient")
		}
		return nil
	}
}

// awaitAlarm waits until the fake clock has at least n pending timers: the
// sleeper of the retry decorator parked on its backoff.
func awaitAlarm(t *testing.T, c *clock.Fake, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for c.Pending() < n {
		if time.Now().After(deadline) {
			t.Fatalf("the clock has %d pending alarms, want %d", c.Pending(), n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestComposeRetriesAnIdempotentMethodOnADeclaredCode(t *testing.T) {
	c := clock.NewFake(start)
	interceptor, err := provider.ComposeUnaryInterceptor(composeConfig(c, true, codes.Unavailable))
	if err != nil {
		t.Fatalf("ComposeUnaryInterceptor() = %v, want nil", err)
	}
	ctx, cancel := budgeted(c)
	defer cancel()

	invoked := 0
	done := make(chan error, 1)
	go func() { done <- interceptor(ctx, checkMethod, nil, nil, nil, failing(1, codes.Unavailable, &invoked)) }()
	awaitAlarm(t, c, 1)
	c.Advance(100 * time.Millisecond)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("interceptor = %v, want nil after one retry", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the interceptor never returned after the backoff elapsed")
	}
	if invoked != 2 {
		t.Fatalf("invoked %d times, want 2", invoked)
	}
}

func TestComposeMakesOneAttemptWhenRetryIsNotAllowed(t *testing.T) {
	c := clock.NewFake(start)
	cases := map[string]struct {
		cfg    provider.Config
		budget bool
	}{
		"without a budget in the context (RES-31)": {composeConfig(c, true, codes.Unavailable), false},
		"method not idempotent (GRP-08)":           {composeConfig(c, false, codes.Unavailable), true},
		"no retryable code declared (GRP-09)":      {composeConfig(c, true), true},
		"code outside the declared list":           {composeConfig(c, true, codes.ResourceExhausted), true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			interceptor, err := provider.ComposeUnaryInterceptor(tc.cfg)
			if err != nil {
				t.Fatalf("ComposeUnaryInterceptor() = %v, want nil", err)
			}
			var ctx context.Context
			var cancel context.CancelFunc
			if tc.budget {
				ctx, cancel = budgeted(c)
			} else {
				ctx, cancel = c.WithTimeout(context.Background(), 30*time.Second)
			}
			defer cancel()

			invoked := 0
			err = interceptor(ctx, checkMethod, nil, nil, nil, failing(5, codes.Unavailable, &invoked))
			if status.Code(err) != codes.Unavailable {
				t.Fatalf("interceptor = %v, want the Unavailable status back", err)
			}
			if invoked != 1 {
				t.Fatalf("invoked %d times, want 1", invoked)
			}
		})
	}
}

func TestComposeRefusesAnUndeclaredMethodWithoutInvoking(t *testing.T) {
	c := clock.NewFake(start)
	interceptor, err := provider.ComposeUnaryInterceptor(composeConfig(c, true))
	if err != nil {
		t.Fatalf("ComposeUnaryInterceptor() = %v, want nil", err)
	}
	ctx, cancel := budgeted(c)
	defer cancel()

	invoked := 0
	err = interceptor(ctx, "/orders.v1.Orders/Place", nil, nil, nil, failing(0, codes.OK, &invoked))
	if !errors.Is(err, provider.ErrMethodNotDeclared) {
		t.Fatalf("interceptor = %v, want ErrMethodNotDeclared", err)
	}
	if invoked != 0 {
		t.Fatal("the invoker ran for an undeclared method")
	}
}

func TestComposeRefusesASheetWhosePositionHasNoDecorator(t *testing.T) {
	c := clock.NewFake(start)
	cfg := composeConfig(c, true)
	cfg.Sheet.RateLimit = resilience.Declare(resilience.RateLimitPolicy{PerSecond: 10, Burst: 10})

	if _, err := provider.ComposeUnaryInterceptor(cfg); !errors.Is(err, resilience.ErrBlankField) {
		t.Fatalf("ComposeUnaryInterceptor() = %v, want ErrBlankField: no rate-limit decorator exists to fill a declared position (RES-21)", err)
	}
}

func TestComposeWithRetryDeclaredOffMakesOneAttempt(t *testing.T) {
	c := clock.NewFake(start)
	cfg := composeConfig(c, true, codes.Unavailable)
	cfg.Sheet.Retry = resilience.Declare(false)
	interceptor, err := provider.ComposeUnaryInterceptor(cfg)
	if err != nil {
		t.Fatalf("ComposeUnaryInterceptor() = %v, want nil", err)
	}
	ctx, cancel := budgeted(c)
	defer cancel()

	invoked := 0
	if err := interceptor(ctx, checkMethod, nil, nil, nil, failing(5, codes.Unavailable, &invoked)); status.Code(err) != codes.Unavailable {
		t.Fatalf("interceptor = %v, want Unavailable", err)
	}
	if invoked != 1 {
		t.Fatalf("invoked %d times, want 1", invoked)
	}
}

func TestStatusClassifier(t *testing.T) {
	classify := provider.StatusClassifier([]codes.Code{codes.Unavailable, codes.DeadlineExceeded})
	cases := map[string]struct {
		err  error
		want retry.Retryability
	}{
		"declared code":   {status.Error(codes.Unavailable, "x"), retry.Retryable},
		"other declared":  {status.Error(codes.DeadlineExceeded, "x"), retry.Retryable},
		"undeclared code": {status.Error(codes.InvalidArgument, "x"), retry.NotRetryable},
		"plain error":     {errors.New("boom"), retry.NotRetryable},
		"nil":             {nil, retry.NotRetryable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := classify(tc.err); got != tc.want {
				t.Fatalf("classify = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestComposeObservesTheCall(t *testing.T) {
	c := clock.NewFake(start)
	cfg := composeConfig(c, true)

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	cfg.Tracer = tp.Tracer("test")

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	instruments, err := metrics.New(mp.Meter("test"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Instruments = instruments

	interceptor, err := provider.ComposeUnaryInterceptor(cfg)
	if err != nil {
		t.Fatalf("ComposeUnaryInterceptor() = %v, want nil", err)
	}
	ctx, cancel := budgeted(c)
	defer cancel()

	invoked := 0
	_ = interceptor(ctx, checkMethod, nil, nil, nil, failing(5, codes.Unavailable, &invoked))

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("exported %d spans, want 1", len(spans))
	}
	span := spans[0]
	if span.Name != "dmpf.grpc.client "+checkMethod {
		t.Errorf("span name = %q", span.Name)
	}
	attrs := map[string]string{}
	for _, kv := range span.Attributes {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	if attrs["dmpf.dependency"] != "orders" || attrs["dmpf.operation"] != checkMethod || attrs["dmpf.service"] != "checkout" {
		t.Errorf("span attributes = %v, want dependency/operation/service", attrs)
	}
	if attrs["dmpf.error.category"] != "unavailable" || attrs["dmpf.outcome_category"] != "unavailable" {
		t.Errorf("span categories = %v, want unavailable (TRC-12)", attrs)
	}
	if span.Status.Description != "" {
		t.Errorf("span status carries a message %q: redaction bypassed (TRC-12)", span.Status.Description)
	}

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	recorded := map[string]bool{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			recorded[m.Name] = true
		}
	}
	for _, name := range []string{metrics.RequestsTotal, metrics.ErrorsTotal, metrics.RequestDurationSeconds} {
		if !recorded[name] {
			t.Errorf("series %q was not recorded (MET-08..10)", name)
		}
	}
}

func breakerFloor(t *testing.T, cfg provider.Config) int {
	t.Helper()
	policy, declared := cfg.Sheet.Breaker.Get()
	if !declared {
		t.Fatal("the default sheet declares no breaker")
	}
	return policy.MinSamples
}

func answering(code codes.Code, invoked *int) grpc.UnaryInvoker {
	return func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		*invoked++
		return status.Error(code, "answer")
	}
}

func TestComposeBreakerDoesNotOpenOnBusinessAnswers(t *testing.T) {
	for _, code := range []codes.Code{codes.NotFound, codes.AlreadyExists, codes.FailedPrecondition, codes.Aborted, codes.InvalidArgument, codes.PermissionDenied, codes.Unauthenticated, codes.OutOfRange, codes.Canceled} {
		t.Run(code.String(), func(t *testing.T) {
			c := clock.NewFake(start)
			cfg := composeConfig(c, false)
			interceptor, err := provider.ComposeUnaryInterceptor(cfg)
			if err != nil {
				t.Fatalf("ComposeUnaryInterceptor() = %v, want nil", err)
			}
			ctx, cancel := budgeted(c)
			defer cancel()

			invoked := 0
			for range breakerFloor(t, cfg) * 2 {
				_ = interceptor(ctx, checkMethod, nil, nil, nil, answering(code, &invoked))
			}
			if err := interceptor(ctx, checkMethod, nil, nil, nil, failing(0, codes.OK, &invoked)); err != nil {
				t.Fatalf("interceptor = %v after a burst of %s, want the call through: an answer is not unavailability (RES-10)", err, code)
			}
		})
	}
}

func TestComposeBreakerOpensOnUnavailability(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Internal, codes.Unknown, codes.DataLoss} {
		t.Run(code.String(), func(t *testing.T) {
			c := clock.NewFake(start)
			cfg := composeConfig(c, false)
			interceptor, err := provider.ComposeUnaryInterceptor(cfg)
			if err != nil {
				t.Fatalf("ComposeUnaryInterceptor() = %v, want nil", err)
			}
			ctx, cancel := budgeted(c)
			defer cancel()

			invoked := 0
			for range breakerFloor(t, cfg) {
				_ = interceptor(ctx, checkMethod, nil, nil, nil, answering(code, &invoked))
			}
			before := invoked
			if err := interceptor(ctx, checkMethod, nil, nil, nil, answering(code, &invoked)); !errors.Is(err, resilience.ErrBreakerOpen) || invoked != before {
				t.Fatalf("interceptor = %v, invoked %d more; want ErrBreakerOpen without invoking (RES-12)", err, invoked-before)
			}
		})
	}
}
