package otelboot_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func validConfig() otelboot.Config {
	return otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{
			ServiceName:       "orders",
			ServiceVersion:    "1.4.2",
			ServiceInstanceID: "orders-7c9f",
		},
	}
}

func TestAConfigWithTheW3CPropagatorAndACompleteResourceIsValid(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestAConfigWithoutAPropagatorIsRefused(t *testing.T) {
	config := validConfig()
	config.Propagator = nil

	err := config.Validate()
	if !errors.Is(err, otelboot.ErrPropagatorRequired) {
		t.Fatalf("Validate() = %v, want ErrPropagatorRequired (TRC-09)", err)
	}
}

func TestAPropagatorThatDoesNotCarryW3CTraceContextIsRefused(t *testing.T) {
	config := validConfig()
	config.Propagator = propagation.Baggage{}

	err := config.Validate()
	if !errors.Is(err, otelboot.ErrPropagatorNotW3C) {
		t.Fatalf("Validate() = %v, want ErrPropagatorNotW3C (TRC-09)", err)
	}
}

func TestTheW3CPropagatorComposedWithBaggageIsAccepted(t *testing.T) {
	config := validConfig()
	config.Propagator = propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	)

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil: composing baggage keeps the trace context", err)
	}
}

func TestAnIncompleteResourceReportsEveryMissingFieldAtOnce(t *testing.T) {
	config := validConfig()
	config.Resource = otelboot.Resource{}

	err := config.Validate()
	if !errors.Is(err, otelboot.ErrResourceIncomplete) {
		t.Fatalf("Validate() = %v, want ErrResourceIncomplete", err)
	}
	for _, key := range []string{
		string(semconv.ServiceNameKey),
		string(semconv.ServiceVersionKey),
		string(semconv.ServiceInstanceIDKey),
	} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("Validate() = %q, want it to name the missing %s", err, key)
		}
	}
}

func TestEachServiceFieldIsRequiredOnItsOwn(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		blank   func(*otelboot.Resource)
		missing string
	}{
		{"no name", func(r *otelboot.Resource) { r.ServiceName = "" }, string(semconv.ServiceNameKey)},
		{"no version", func(r *otelboot.Resource) { r.ServiceVersion = "" }, string(semconv.ServiceVersionKey)},
		{"no instance", func(r *otelboot.Resource) { r.ServiceInstanceID = "" }, string(semconv.ServiceInstanceIDKey)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			config := validConfig()
			testCase.blank(&config.Resource)

			err := config.Validate()
			if !errors.Is(err, otelboot.ErrResourceIncomplete) {
				t.Fatalf("Validate() = %v, want ErrResourceIncomplete", err)
			}
			if !strings.Contains(err.Error(), testCase.missing) {
				t.Errorf("Validate() = %q, want it to name %s", err, testCase.missing)
			}
		})
	}
}

func TestABlankSheetIsRefusedByValidate(t *testing.T) {
	config := validConfig()
	blank := resilience.Defaults("payments")
	blank.Breaker = resilience.Field[resilience.BreakerPolicy]{}
	config.Sheets = []resilience.Sheet{blank}

	err := config.Validate()
	if !errors.Is(err, resilience.ErrBlankField) {
		t.Fatalf("Validate() = %v, want the sheet's own ErrBlankField (RES-21)", err)
	}
}

func TestValidateReportsThePropagatorAndTheResourceInOnePass(t *testing.T) {
	config := otelboot.Config{}

	err := config.Validate()
	if !errors.Is(err, otelboot.ErrPropagatorRequired) || !errors.Is(err, otelboot.ErrResourceIncomplete) {
		t.Fatalf("Validate() = %v, want both faults reported at once", err)
	}
}

func TestSamplingDefaultsToEveryClassAtOne(t *testing.T) {
	rates := validConfig().EffectiveSampling()

	for _, class := range []tracing.Class{tracing.ClassError, tracing.ClassWrite, tracing.ClassRead,
		tracing.ClassMaintenance, tracing.ClassUnclassified} {
		if got := rates.RateFor(class); got != 1 {
			t.Errorf("RateFor(%q) = %v, want 1: the table of TRC-13 lives in the tail only (RF-E5)", class, got)
		}
	}
}

func TestADeclaredSamplingIsKeptAndAnAbsentClassTakesTheMostRestrictiveRate(t *testing.T) {
	config := validConfig()
	config.Sampling = tracing.Rates{tracing.ClassWrite: 0.5}

	rates := config.EffectiveSampling()
	if got := rates.RateFor(tracing.ClassWrite); got != 0.5 {
		t.Errorf("RateFor(write) = %v, want the declared 0.5", got)
	}
	if got := rates.RateFor(tracing.ClassMaintenance); got != tracing.RateMostRestrictive {
		t.Errorf("RateFor(maintenance) = %v, want the most restrictive %v", got, tracing.RateMostRestrictive)
	}
}

func TestTheResourceAttributesIdentifyTheServiceWithTheSemconvKeys(t *testing.T) {
	attributes := index(validConfig().ResourceAttributes())

	for key, want := range map[attribute.Key]string{
		semconv.ServiceNameKey:       "orders",
		semconv.ServiceVersionKey:    "1.4.2",
		semconv.ServiceInstanceIDKey: "orders-7c9f",
	} {
		if got := attributes[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestExtraResourceAttributesAreCarriedThrough(t *testing.T) {
	config := validConfig()
	config.Resource.Attributes = []attribute.KeyValue{attribute.String("deployment.environment.name", "hmg")}

	attributes := index(config.ResourceAttributes())
	if got := attributes["deployment.environment.name"]; got != "hmg" {
		t.Errorf("deployment.environment.name = %q, want %q", got, "hmg")
	}
}

func TestTheSheetsStayOutOfTheResourceAttributes(t *testing.T) {
	config := validConfig()
	payments := resilience.Defaults("payments")
	payments.Deadline = resilience.Declare(3 * time.Second)
	payments.MaxAttempts = resilience.Declare(5)
	config.Sheets = []resilience.Sheet{payments, resilience.Defaults("ledger")}

	for key, value := range index(config.ResourceAttributes()) {
		if strings.HasPrefix(string(key), "dmpf.sheet.") {
			t.Errorf("%s = %q is on the resource, want the sheets only on the resilience sheet in effect record", key, value)
		}
	}
}

func index(attributes []attribute.KeyValue) map[attribute.Key]string {
	indexed := make(map[attribute.Key]string, len(attributes))
	for _, attr := range attributes {
		indexed[attr.Key] = attr.Value.AsString()
	}
	return indexed
}

func resourceOfStarted(t *testing.T, mutate func(*otelboot.Config)) map[attribute.Key]string {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	startedRuntime(t, func(config *otelboot.Config) {
		config.MetricReader = reader
		if mutate != nil {
			mutate(config)
		}
	})
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	if got := collected.Resource.SchemaURL(); got != semconv.SchemaURL {
		t.Errorf("schema_url = %q, want %q", got, semconv.SchemaURL)
	}
	return index(collected.Resource.Attributes())
}

func cleanResourceEnv(t *testing.T) {
	t.Helper()
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "")
}

func TestTheEnvironmentOverridesTheDeclaredResource(t *testing.T) {
	cleanResourceEnv(t)
	t.Setenv("OTEL_SERVICE_NAME", "billing")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.version=9.9.9,deployment.environment.name=hmg")

	attributes := resourceOfStarted(t, nil)

	for key, want := range map[attribute.Key]string{
		semconv.ServiceNameKey:               "billing",
		semconv.ServiceVersionKey:            "9.9.9",
		semconv.ServiceInstanceIDKey:         "orders-7c9f",
		semconv.DeploymentEnvironmentNameKey: "hmg",
	} {
		if got := attributes[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestTheResourceIsValidatedAfterTheEnvironment(t *testing.T) {
	cleanResourceEnv(t)
	config := validConfig()
	config.Resource.ServiceVersion = ""
	if err := config.Validate(); !errors.Is(err, otelboot.ErrResourceIncomplete) {
		t.Fatalf("Validate() = %v, want ErrResourceIncomplete without service.version", err)
	}

	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.version=2.0.0")
	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil once the environment declares service.version", err)
	}
}

func TestTheProcessRoleIsOnTheResource(t *testing.T) {
	cleanResourceEnv(t)

	attributes := resourceOfStarted(t, func(config *otelboot.Config) { config.Resource.Role = "relay" })

	if got := attributes[otelboot.ProcessRoleAttribute]; got != "relay" {
		t.Errorf("%s = %q, want relay", otelboot.ProcessRoleAttribute, got)
	}
}

func TestTheResourceCarriesTheSDKAndTheRuntimeButNotTheHost(t *testing.T) {
	cleanResourceEnv(t)

	attributes := resourceOfStarted(t, nil)

	for _, key := range []attribute.Key{semconv.TelemetrySDKNameKey, semconv.TelemetrySDKLanguageKey,
		semconv.TelemetrySDKVersionKey, semconv.ProcessRuntimeNameKey, semconv.ProcessRuntimeVersionKey} {
		if attributes[key] == "" {
			t.Errorf("%s is absent from the resource", key)
		}
	}
	for _, key := range []attribute.Key{semconv.HostNameKey, semconv.ContainerIDKey} {
		if _, present := attributes[key]; present {
			t.Errorf("%s is on the resource, want it out", key)
		}
	}
}
