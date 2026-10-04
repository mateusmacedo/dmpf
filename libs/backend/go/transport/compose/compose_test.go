package compose_test

import (
	"context"
	"errors"
	"maps"
	"runtime"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/compose"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

var errTransient = errors.New("transient")

func config(c *clock.Fake) compose.Config {
	sheet := resilience.Defaults("dep")
	sheet.Backoff = resilience.Declare(resilience.BackoffPolicy{Base: time.Millisecond, Factor: 2, Cap: 10 * time.Millisecond})
	return compose.Config{
		Sheet: sheet, Clock: c,
		Rand:       func() float64 { return 0 },
		Category:   func(error) string { return "transient" },
		Classifier: func(err error) retry.Retryability { return retry.Retryable },
	}
}

func TestBuildRetriesUnderTheConjunction(t *testing.T) {
	c := clock.NewFake(start)
	call, err := compose.Build(config(c))
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = retry.WithBudget(ctx, retry.WithTotal(10*time.Second))

	attempts := 0
	err = call(ctx, compose.Operation(config(c), "op", true), func(context.Context) error {
		attempts++
		if attempts == 1 {
			return errTransient
		}
		return nil
	})
	if err != nil || attempts != 2 {
		t.Fatalf("call = %v after %d attempts, want nil after 2", err, attempts)
	}
}

func TestBuildWithoutBudgetMakesOneAttempt(t *testing.T) {
	c := clock.NewFake(start)
	call, _ := compose.Build(config(c))
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	attempts := 0
	_ = call(ctx, compose.Operation(config(c), "op", true), func(context.Context) error { attempts++; return errTransient })
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 without retry.WithBudget (RES-31)", attempts)
	}
}

func TestRetryDeclaredOffIsTheIdentity(t *testing.T) {
	c := clock.NewFake(start)
	cfg := config(c)
	cfg.Sheet.Retry = resilience.Declare(false)
	call, err := compose.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = retry.WithBudget(ctx, retry.WithTotal(10*time.Second))
	attempts := 0
	_ = call(ctx, compose.Operation(cfg, "op", true), func(context.Context) error { attempts++; return errTransient })
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 with retry declared off", attempts)
	}
}

func TestBuildRefusesADeclaredPositionWithoutDecorator(t *testing.T) {
	c := clock.NewFake(start)
	cfg := config(c)
	cfg.Sheet.RateLimit = resilience.Declare(resilience.RateLimitPolicy{PerSecond: 1, Burst: 1})
	if _, err := compose.Build(cfg); !errors.Is(err, resilience.ErrBlankField) {
		t.Fatalf("Build() = %v, want ErrBlankField (RES-21)", err)
	}
}

func TestOperationIsBoundedByTheSheet(t *testing.T) {
	c := clock.NewFake(start)
	op := compose.Operation(config(c), "publish x", true)
	if op.Deadline != resilience.DefaultDeadline || op.EstimatedDuration != resilience.DefaultDeadline/4 || !op.Idempotent || op.Kind != resilience.Remote {
		t.Fatalf("Operation = %+v", op)
	}
	if err := op.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

var errAnswer = errors.New("answer")

func breakerFloor(t *testing.T, cfg compose.Config) int {
	t.Helper()
	policy, declared := cfg.Sheet.Breaker.Get()
	if !declared {
		t.Fatal("the default sheet declares no breaker")
	}
	return policy.MinSamples
}

func TestBuildHandsTheFailureClassifierToTheBreaker(t *testing.T) {
	c := clock.NewFake(start)
	cfg := config(c)
	cfg.BreakerFailure = func(err error) bool { return !errors.Is(err, errAnswer) }
	call, err := compose.Build(cfg)
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	op := compose.Operation(cfg, "op", false)

	for range breakerFloor(t, cfg) * 2 {
		_ = call(ctx, op, func(context.Context) error { return errAnswer })
	}

	invoked := false
	if err := call(ctx, op, func(context.Context) error { invoked = true; return nil }); err != nil || !invoked {
		t.Fatalf("call = %v, invoked %v; want the call through: an uncounted error must not open the breaker", err, invoked)
	}
}

func TestBuildWithoutFailureClassifierCountsEveryError(t *testing.T) {
	c := clock.NewFake(start)
	cfg := config(c)
	call, err := compose.Build(cfg)
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	op := compose.Operation(cfg, "op", false)

	for range breakerFloor(t, cfg) {
		_ = call(ctx, op, func(context.Context) error { return errAnswer })
	}

	if err := call(ctx, op, func(context.Context) error { return nil }); !errors.Is(err, resilience.ErrBreakerOpen) {
		t.Fatalf("call = %v, want ErrBreakerOpen", err)
	}
}

func TestTheComposedBreakerLogsItsTransitionOnTheConfiguredLogger(t *testing.T) {
	cases := map[string]func(t *testing.T, cfg *compose.Config, sink log.LoggerProvider){
		"configured logger": func(_ *testing.T, cfg *compose.Config, sink log.LoggerProvider) { cfg.LoggerProvider = sink },
		"no logger": func(t *testing.T, _ *compose.Config, sink log.LoggerProvider) {
			previous := otel.GetLoggerProvider()
			otel.SetLoggerProvider(sink)
			t.Cleanup(func() { otel.SetLoggerProvider(previous) })
		},
	}
	for name, configure := range cases {
		t.Run(name, func(t *testing.T) {
			exporter := &memoryExporter{}
			c := clock.NewFake(start)
			cfg := config(c)
			configure(t, &cfg, sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter))))
			call, err := compose.Build(cfg)
			if err != nil {
				t.Fatalf("Build() = %v", err)
			}
			ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			op := compose.Operation(cfg, "op", false)

			for range breakerFloor(t, cfg) {
				_ = call(ctx, op, func(context.Context) error { return errAnswer })
			}

			var transitions []map[string]any
			for _, record := range exporter.maps() {
				if record["msg"] == "resilience: breaker state changed" {
					transitions = append(transitions, record)
				}
			}
			if len(transitions) != 1 {
				t.Fatalf("transitions = %v in %v, want the one to open", transitions, exporter.maps())
			}
			want := map[string]any{
				"level": "WARN", "dmpf.dependency": "dep", "dmpf.breaker.state": "open",
				"scope": "github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience",
			}
			for key, value := range want {
				if transitions[0][key] != value {
					t.Errorf("transition %v: %s = %v, want %v", transitions[0], key, transitions[0][key], value)
				}
			}
		})
	}
}

func TestBuildOpensOneResilienceSpanOverTheRetries(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	c := clock.NewFake(start)
	cfg := config(c)
	cfg.Tracer = tp.Tracer("t")
	call, err := compose.Build(cfg)
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = retry.WithBudget(ctx, retry.WithTotal(10*time.Second))

	attempts := 0
	err = call(ctx, compose.Operation(cfg, "op", true), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errTransient
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("call = %v after %d attempts, want nil after 3", err, attempts)
	}

	ended := recorder.Ended()
	if len(ended) != 1 || ended[0].Name() != "dmpf.resilience dep" || ended[0].SpanKind() != trace.SpanKindInternal {
		t.Fatalf("spans = %v, want one INTERNAL dmpf.resilience dep over the attempts", ended)
	}
	retries := 0
	for _, event := range ended[0].Events() {
		if event.Name == "dmpf.retry.attempt" {
			retries++
		}
	}
	if retries != 2 {
		t.Errorf("%d retry events, want 2", retries)
	}
	maxAttempts := -1
	for _, kv := range ended[0].Attributes() {
		if kv.Key == "dmpf.retry.max_attempts" {
			maxAttempts = int(kv.Value.AsInt64())
		}
	}
	if maxAttempts != resilience.DefaultMaxAttempts {
		t.Errorf("dmpf.retry.max_attempts = %d, want the sheet's %d", maxAttempts, resilience.DefaultMaxAttempts)
	}
}

func TestRetryDeclaredOffMakesASingleAttemptTheCeiling(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	c := clock.NewFake(start)
	cfg := config(c)
	cfg.Tracer = tp.Tracer("t")
	cfg.Sheet.Retry = resilience.Declare(false)
	call, err := compose.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = call(ctx, compose.Operation(cfg, "op", true), func(context.Context) error { return nil })

	for _, kv := range recorder.Ended()[0].Attributes() {
		if kv.Key == "dmpf.retry.max_attempts" && kv.Value.AsInt64() == 1 {
			return
		}
	}
	t.Fatalf("attributes = %v, want dmpf.retry.max_attempts=1 with retry declared off", recorder.Ended()[0].Attributes())
}

func TestBuildRecordsNoClientREDOfItsOwn(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	instruments, err := metrics.New(mp.Meter("t"))
	if err != nil {
		t.Fatal(err)
	}
	c := clock.NewFake(start)
	cfg := config(c)
	cfg.Instruments = instruments
	call, err := compose.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = call(ctx, compose.Operation(cfg, "op", false), func(context.Context) error { return nil })
	_ = call(ctx, compose.Operation(cfg, "op", false), func(context.Context) error { return errTransient })

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			switch m.Name {
			case "dmpf_service_requests_total", "dmpf_service_errors_total", metrics.RequestDurationSeconds:
				t.Errorf("series %s recorded by the composition: the client RED comes from otelgrpc and otelhttp (RF-D2)", m.Name)
			}
		}
	}
}

type unavailable struct{}

func (unavailable) Error() string         { return "unavailable: secret detail" }
func (unavailable) ErrorCategory() string { return "unavailable" }
func (unavailable) ErrorCode() string     { return "TEST-01" }

func spanRecorder(t *testing.T) (*tracetest.SpanRecorder, trace.Tracer) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return recorder, tp.Tracer("t")
}

func eventsNamed(span sdktrace.ReadOnlySpan, name string) []map[string]any {
	var found []map[string]any
	for _, event := range span.Events() {
		if event.Name != name {
			continue
		}
		attrs := map[string]any{}
		for _, kv := range event.Attributes {
			attrs[string(kv.Key)] = kv.Value.AsInterface()
		}
		found = append(found, attrs)
	}
	return found
}

func awaitAlarm(t *testing.T, c *clock.Fake, want int) {
	t.Helper()
	limit := time.Now().Add(2 * time.Second)
	for c.Pending() != want {
		if time.Now().After(limit) {
			t.Fatalf("Pending() = %d, want %d", c.Pending(), want)
		}
		runtime.Gosched()
	}
}

func TestTheDeclaredDegradationLeavesItsEventOnTheResilienceSpan(t *testing.T) {
	compositions := map[string]func(t *testing.T, cfg compose.Config) resilience.Call{
		"Build": func(t *testing.T, cfg compose.Config) resilience.Call {
			call, err := compose.Build(cfg)
			if err != nil {
				t.Fatalf("Build() = %v", err)
			}
			return call
		},
		"Shared": func(t *testing.T, cfg compose.Config) resilience.Call {
			slots := compose.Shared(cfg)
			retrying, err := compose.Retry(cfg, cfg.Classifier)
			if err != nil {
				t.Fatalf("Retry() = %v", err)
			}
			slots.Retry = retrying
			call, err := resilience.Compose(cfg.Sheet, slots)
			if err != nil {
				t.Fatalf("Compose() = %v", err)
			}
			return call
		},
	}
	for name, composition := range compositions {
		t.Run(name, func(t *testing.T) {
			spans, tracer := spanRecorder(t)
			cfg := config(clock.NewFake(start))
			cfg.Tracer = tracer
			cfg.Sheet.Degradation = resilience.Declare(resilience.Degrade)
			call := composition(t, cfg)

			err := call(context.Background(), compose.Operation(cfg, "op", false), func(context.Context) error { return errTransient })

			var degraded *resilience.DegradedResult
			if !errors.As(err, &degraded) || !errors.Is(err, errTransient) {
				t.Fatalf("call = %v, want a *DegradedResult over the failure (RES-38)", err)
			}
			ended := spans.Ended()
			if len(ended) != 1 || ended[0].Name() != "dmpf.resilience dep" {
				t.Fatalf("spans = %v, want the one dmpf.resilience dep", ended)
			}
			events := eventsNamed(ended[0], "dmpf.degraded")
			if len(events) != 1 || events[0]["dmpf.error.code"] != "RES-37" {
				t.Fatalf("events = %v, want one dmpf.degraded with dmpf.error.code=RES-37 on the resilience span (RF-B5)", ended[0].Events())
			}
		})
	}
}

func TestBuildRefusesADegradationModeTheCompositionCannotHonour(t *testing.T) {
	cases := map[resilience.Degradation]error{
		resilience.Defer: resilience.ErrDeferIsOutbox,
		"liquidar":       resilience.ErrBlankField,
	}
	for mode, want := range cases {
		t.Run(string(mode), func(t *testing.T) {
			cfg := config(clock.NewFake(start))
			cfg.Sheet.Degradation = resilience.Declare(mode)
			if _, err := compose.Build(cfg); !errors.Is(err, want) {
				t.Fatalf("Build() = %v, want %v (RES-37)", err, want)
			}
		})
	}
}

func TestUnderTheOwnedSendTheRealDecoratorsLeaveTheirEventsOnTheSend(t *testing.T) {
	cases := map[string]struct {
		configure func(*compose.Config)
		run       func(t *testing.T, c *clock.Fake, call resilience.Call, op resilience.Operation, send context.Context)
		event     string
		want      []map[string]any
	}{
		"retry": {
			run: func(t *testing.T, c *clock.Fake, call resilience.Call, op resilience.Operation, send context.Context) {
				ctx, cancel := c.WithTimeout(send, 30*time.Second)
				defer cancel()
				attempts := 0
				err := call(retry.WithBudget(ctx, retry.WithTotal(10*time.Second)), op, func(context.Context) error {
					attempts++
					if attempts < 3 {
						return unavailable{}
					}
					return nil
				})
				if err != nil || attempts != 3 {
					t.Fatalf("call = %v after %d attempts, want nil after 3", err, attempts)
				}
			},
			event: "dmpf.retry.attempt",
			want: []map[string]any{
				{"dmpf.retry.attempt": int64(2), "dmpf.retry.previous_category": "unavailable"},
				{"dmpf.retry.attempt": int64(3), "dmpf.retry.previous_category": "unavailable"},
			},
		},
		"breaker open": {
			configure: func(cfg *compose.Config) {
				cfg.Sheet.Breaker = resilience.Declare(resilience.BreakerPolicy{Window: time.Minute, Threshold: 0.5, MinSamples: 1, Cooldown: time.Minute, Probes: 1})
			},
			run: func(t *testing.T, _ *clock.Fake, call resilience.Call, op resilience.Operation, send context.Context) {
				_ = call(context.Background(), op, func(context.Context) error { return errTransient })
				if err := call(send, op, func(context.Context) error { return nil }); !errors.Is(err, resilience.ErrBreakerOpen) {
					t.Fatalf("call = %v, want ErrBreakerOpen", err)
				}
			},
			event: "dmpf.breaker.rejected",
			want:  []map[string]any{{"dmpf.dependency": "dep"}},
		},
		"bulkhead saturated": {
			configure: func(cfg *compose.Config) {
				cfg.Sheet.Bulkhead = resilience.Declare(resilience.BulkheadPolicy{Pool: 1, Queue: 1, Acquisition: time.Second})
			},
			run: func(t *testing.T, c *clock.Fake, call resilience.Call, op resilience.Operation, send context.Context) {
				entered, release := make(chan struct{}, 2), make(chan struct{})
				hold := func(context.Context) error {
					entered <- struct{}{}
					<-release
					return nil
				}
				var holders sync.WaitGroup
				holders.Add(2)
				go func() { defer holders.Done(); _ = call(context.Background(), op, hold) }()
				select {
				case <-entered:
				case <-time.After(2 * time.Second):
					t.Fatal("the first call never took the pool")
				}
				queued := c.Pending() + 1
				go func() { defer holders.Done(); _ = call(context.Background(), op, hold) }()
				awaitAlarm(t, c, queued)

				err := call(send, op, hold)
				close(release)
				holders.Wait()
				if !errors.Is(err, resilience.ErrBulkheadSaturated) {
					t.Fatalf("call = %v, want ErrBulkheadSaturated with the pool and its queue taken", err)
				}
			},
			event: "dmpf.bulkhead.saturated",
			want:  []map[string]any{{"dmpf.dependency": "dep"}},
		},
		"degraded": {
			configure: func(cfg *compose.Config) { cfg.Sheet.Degradation = resilience.Declare(resilience.Degrade) },
			run: func(t *testing.T, _ *clock.Fake, call resilience.Call, op resilience.Operation, send context.Context) {
				var degraded *resilience.DegradedResult
				if err := call(send, op, func(context.Context) error { return errTransient }); !errors.As(err, &degraded) {
					t.Fatalf("call = %v, want a *DegradedResult", err)
				}
			},
			event: "dmpf.degraded",
			want:  []map[string]any{{"dmpf.error.code": "RES-37"}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			spans, tracer := spanRecorder(t)
			c := clock.NewFake(start)
			cfg := config(c)
			cfg.Tracer = tracer
			if tc.configure != nil {
				tc.configure(&cfg)
			}
			call, err := compose.Build(cfg)
			if err != nil {
				t.Fatalf("Build() = %v", err)
			}
			ctx, send := tracer.Start(context.Background(), "send dep", trace.WithSpanKind(trace.SpanKindProducer))
			tc.run(t, c, call, compose.Operation(cfg, "op", true), tracing.WithOwnedSpan(ctx, send))
			send.End()

			var owned sdktrace.ReadOnlySpan
			for _, span := range spans.Ended() {
				switch {
				case span.SpanContext().SpanID() == send.SpanContext().SpanID():
					owned = span
				case span.Parent().SpanID() == send.SpanContext().SpanID():
					t.Errorf("span %q opened under the owned send, want the resilience recorded on it (TRC-11)", span.Name())
				}
			}
			if owned == nil {
				t.Fatal("the owned send never ended")
			}
			events := eventsNamed(owned, tc.event)
			if len(events) != len(tc.want) {
				t.Fatalf("send events = %v, want %d %s (RF-B5)", owned.Events(), len(tc.want), tc.event)
			}
			for i, want := range tc.want {
				for key, value := range want {
					if events[i][key] != value {
						t.Errorf("event %s %d = %v, want %s=%v", tc.event, i, events[i], key, value)
					}
				}
			}
		})
	}
}

func TestTheRetryEventNumbersTheAttemptAsTheLogDoes(t *testing.T) {
	spans, tracer := spanRecorder(t)
	exporter := &memoryExporter{}
	c := clock.NewFake(start)
	cfg := config(c)
	cfg.Tracer = tracer
	cfg.LoggerProvider = sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	call, err := compose.Build(cfg)
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	attempts := 0
	err = call(retry.WithBudget(ctx, retry.WithTotal(10*time.Second)), compose.Operation(cfg, "op", true), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errTransient
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("call = %v after %d attempts, want nil after 3", err, attempts)
	}

	logged := map[int64]string{}
	for _, record := range exporter.maps() {
		if number, ok := record["dmpf.retry.attempt"].(int64); ok {
			logged[number] = record["msg"].(string)
		}
	}
	events := eventsNamed(spans.Ended()[0], "dmpf.retry.attempt")
	if len(events) != attempts-1 {
		t.Fatalf("events = %v, want one per repeated attempt", events)
	}
	for _, event := range events {
		number, _ := event["dmpf.retry.attempt"].(int64)
		if _, ran := logged[number]; !ran || logged[number-1] != "transport: call failed" {
			t.Errorf("event dmpf.retry.attempt=%d, want the number the log gives the attempt it starts, after a failed one: %v (TRC-11)", number, logged)
		}
	}
}

type rpcStatus struct{ code int }

func (rpcStatus) Error() string { return "rpc error: code = Unavailable desc = secret detail" }

const codeUnavailable = 14

func unavailableStatus(err error) bool {
	var status rpcStatus
	return errors.As(err, &status) && status.code == codeUnavailable
}

func TestARetriedStatusIsCountedUnderTheCategoryItsSpanRecords(t *testing.T) {
	statusClassifier := func(err error) retry.Retryability {
		if unavailableStatus(err) {
			return retry.Retryable
		}
		return retry.NotRetryable
	}
	compositions := map[string]func(cfg compose.Config) (resilience.Call, error){
		"Build": func(cfg compose.Config) (resilience.Call, error) {
			cfg.Classifier = statusClassifier
			return compose.Build(cfg)
		},
		"Retry per method": func(cfg compose.Config) (resilience.Call, error) {
			cfg.Classifier = nil
			slots := compose.Shared(cfg)
			retryDecorator, err := compose.Retry(cfg, statusClassifier)
			if err != nil {
				return nil, err
			}
			slots.Retry = retryDecorator
			return resilience.Compose(cfg.Sheet, slots)
		},
	}
	for name, composition := range compositions {
		t.Run(name, func(t *testing.T) {
			reader := sdkmetric.NewManualReader()
			meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			t.Cleanup(func() { _ = meterProvider.Shutdown(context.Background()) })
			instruments, err := metrics.New(meterProvider.Meter("t"))
			if err != nil {
				t.Fatal(err)
			}
			spans, tracer := spanRecorder(t)
			c := clock.NewFake(start)
			cfg := config(c)
			cfg.Tracer = tracer
			cfg.Instruments = instruments
			cfg.Category = func(err error) string {
				if unavailableStatus(err) {
					return "TransientDependency"
				}
				return "_OTHER"
			}
			call, err := composition(cfg)
			if err != nil {
				t.Fatalf("composition = %v", err)
			}
			ctx, cancel := c.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			_ = call(retry.WithBudget(ctx, retry.WithTotal(10*time.Second)), compose.Operation(cfg, "op", true), func(context.Context) error {
				return rpcStatus{code: codeUnavailable}
			})

			ended := spans.Ended()
			if len(ended) != 1 {
				t.Fatalf("spans = %v, want the one dmpf.resilience dep", ended)
			}
			recorded := ""
			for _, kv := range ended[0].Attributes() {
				if kv.Key == "error.type" {
					recorded = kv.Value.AsString()
				}
			}
			if recorded != "TransientDependency" {
				t.Fatalf("span error.type = %q, want the transport's category TransientDependency", recorded)
			}
			repeated := resilience.DefaultMaxAttempts - 1
			events := eventsNamed(ended[0], "dmpf.retry.attempt")
			if len(events) != repeated {
				t.Fatalf("events = %v, want %d repeated attempts", events, repeated)
			}
			for _, event := range events {
				if got := event["dmpf.retry.previous_category"]; got != recorded {
					t.Errorf("dmpf.retry.previous_category = %v, want %q, the error.type of its span (RF-B1)", got, recorded)
				}
			}

			var collected metricdata.ResourceMetrics
			if err := reader.Collect(context.Background(), &collected); err != nil {
				t.Fatal(err)
			}
			counted := map[string]int64{}
			for _, scope := range collected.ScopeMetrics {
				for _, m := range scope.Metrics {
					if m.Name != metrics.RetriesTotal {
						continue
					}
					for _, point := range m.Data.(metricdata.Sum[int64]).DataPoints {
						category, _ := point.Attributes.Value(attribute.Key(metrics.KeyErrorType))
						counted[category.AsString()] += point.Value
					}
				}
			}
			if want := map[string]int64{recorded: int64(repeated)}; !maps.Equal(counted, want) {
				t.Errorf("%s by %s = %v, want %v, the error.type of its span (RF-B1)", metrics.RetriesTotal, metrics.KeyErrorType, counted, want)
			}
		})
	}
}
