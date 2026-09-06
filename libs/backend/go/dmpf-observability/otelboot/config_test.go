package otelboot_test

import (
	"crypto/tls"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/otelboot"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
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

func TestAnInsecureTransportIsRefusedUnlessItWasAllowed(t *testing.T) {
	config := validConfig()
	config.Transport = otelboot.Transport{Endpoint: "collector:4317", Insecure: true}

	err := config.Validate()
	if !errors.Is(err, otelboot.ErrInsecureNotAllowed) {
		t.Fatalf("Validate() = %v, want ErrInsecureNotAllowed", err)
	}

	config.AllowInsecure = true
	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil once the insecure transport is declared", err)
	}
}

func TestATLSTransportNeedsNoAllowance(t *testing.T) {
	config := validConfig()
	config.Transport = otelboot.Transport{
		Endpoint: "collector:4317",
		TLS:      &tls.Config{MinVersion: tls.VersionTLS13},
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestABlankSheetIsRefusedBeforeItReachesTheResource(t *testing.T) {
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

func TestSamplingDefaultsToTheBaselineOfTRC13(t *testing.T) {
	rates := validConfig().EffectiveSampling()

	want := tracing.DefaultRates()
	for class, rate := range want {
		if got := rates.RateFor(class); got != rate {
			t.Errorf("RateFor(%q) = %v, want %v", class, got, rate)
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

func TestTheEffectiveSheetsBecomeResourceAttributes(t *testing.T) {
	config := validConfig()
	sheet := resilience.Defaults("payments")
	sheet.Deadline = resilience.Declare(3 * time.Second)
	config.Sheets = []resilience.Sheet{sheet}

	attributes := index(config.ResourceAttributes())

	deadline := attribute.Key(otelboot.SheetAttributePrefix + "payments." + resilience.FieldDeadline)
	if got := attributes[deadline]; got != "3s" {
		t.Errorf("%s = %q, want %q (RES-40)", deadline, got, "3s")
	}

	rateLimit := attribute.Key(otelboot.SheetAttributePrefix + "payments." + resilience.FieldRateLimit)
	if got := attributes[rateLimit]; !strings.HasPrefix(got, resilience.NotApplicablePrefix) {
		t.Errorf("%s = %q, want the declared absence to say why the position is empty", rateLimit, got)
	}
}

func TestTwoSheetsDoNotCollideOnTheSameAttribute(t *testing.T) {
	config := validConfig()
	payments := resilience.Defaults("payments")
	payments.MaxAttempts = resilience.Declare(5)
	config.Sheets = []resilience.Sheet{payments, resilience.Defaults("ledger")}

	attributes := index(config.ResourceAttributes())

	if got := attributes[attribute.Key(otelboot.SheetAttributePrefix+"payments."+resilience.FieldMaxAttempts)]; got != "5" {
		t.Errorf("payments max_attempts = %q, want %q", got, "5")
	}
	if got := attributes[attribute.Key(otelboot.SheetAttributePrefix+"ledger."+resilience.FieldMaxAttempts)]; got != "3" {
		t.Errorf("ledger max_attempts = %q, want the platform default %q", got, "3")
	}
}

func index(attributes []attribute.KeyValue) map[attribute.Key]string {
	indexed := make(map[attribute.Key]string, len(attributes))
	for _, attr := range attributes {
		indexed[attr.Key] = attr.Value.AsString()
	}
	return indexed
}
