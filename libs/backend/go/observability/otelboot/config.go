package otelboot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

// The configuration faults the bootstrap refuses. Each one is a decision the
// caller has to make explicitly: a default here would be a decision nobody
// took.
var (
	// ErrPropagatorRequired is a configuration with no propagator. Context
	// propagation is declared, never inherited from the global default
	// (TRC-09).
	ErrPropagatorRequired = errors.New("otelboot: the configuration declares no propagator")

	// ErrPropagatorNotW3C is a propagator that does not carry W3C Trace
	// Context. The platform's wire format is traceparent/tracestate (TRC-09).
	ErrPropagatorNotW3C = errors.New("otelboot: the propagator does not carry W3C Trace Context")

	// ErrResourceIncomplete is a resource that does not identify the service.
	ErrResourceIncomplete = errors.New("otelboot: the resource does not identify the service")
)

// The W3C Trace Context headers a conforming propagator injects. The check is
// on the fields the propagator declares, not on its type, so a composite that
// adds baggage still passes.
const (
	headerTraceparent = "traceparent"
	headerTracestate  = "tracestate"
)

// SheetAttributePrefix opens the key of a field on the resilience sheet in
// effect record: <prefix><field>, beside dmpf.dependency (RES-40).
const SheetAttributePrefix = "dmpf.sheet."

// ProcessRoleAttribute is the role of the process on the resource: api, relay
// or consumer.
const ProcessRoleAttribute = "dmpf.process.role"

// Resource is the identity of the process behind the telemetry. The three
// service fields are mandatory because a signal nobody can attribute to a
// service, a version and an instance is a signal nobody can act on.
type Resource struct {
	ServiceName       string
	ServiceVersion    string
	ServiceInstanceID string
	Role              string

	// Attributes are further resource attributes, such as the deployment
	// environment, that the caller adds on its own terms.
	Attributes []attribute.KeyValue
}

// Config is everything the bootstrap needs. The exporters are injected so the
// suite runs the whole pipeline in memory, and boot supplies the production
// ones from the OTEL_* environment. Disabled (OTEL_SDK_DISABLED) exports nothing.
type Config struct {
	Disabled       bool
	Propagator     propagation.TextMapPropagator
	Resource       Resource
	Sampling       tracing.Rates
	Class          tracing.Class
	Rand           func() float64
	Sheets         []resilience.Sheet
	TraceExporter  sdktrace.SpanExporter
	MetricReader   sdkmetric.Reader
	LoggerProvider *sdklog.LoggerProvider
	Logger         *slog.Logger
	LogLevel       slog.Leveler
}

// Validate reports every fault at once, so a reader fixes the configuration in
// one pass instead of discovering the next fault on the next run.
func (c Config) Validate() error {
	faults := []error{c.validatePropagator(), c.validateResource()}
	for _, sheet := range c.Sheets {
		faults = append(faults, sheet.Validate())
	}
	return errors.Join(faults...)
}

// EnvPropagators may only be absent or tracecontext: baggage would carry the
// tenant out of the process, and none or a third-party format breaks the
// explicit W3C Trace Context of TRC-09 (RF-E4).
const EnvPropagators = "OTEL_PROPAGATORS"

func (c Config) validatePropagator() error {
	if declared := os.Getenv(EnvPropagators); declared != "" && declared != "tracecontext" {
		return fmt.Errorf("%w: %s=%q", ErrPropagatorNotW3C, EnvPropagators, declared)
	}
	if c.Propagator == nil {
		return ErrPropagatorRequired
	}
	fields := c.Propagator.Fields()
	if !slices.Contains(fields, headerTraceparent) || !slices.Contains(fields, headerTracestate) {
		return fmt.Errorf("%w: it injects %v", ErrPropagatorNotW3C, fields)
	}
	return nil
}

func (c Config) validateResource() error {
	resource, err := c.resource()
	if err != nil {
		return err
	}
	identity := resource.Set()
	missing := make([]string, 0, 3)
	for _, key := range []attribute.Key{semconv.ServiceNameKey, semconv.ServiceVersionKey, semconv.ServiceInstanceIDKey} {
		if value, present := identity.Value(key); !present || value.AsString() == "" {
			missing = append(missing, string(key))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	slices.Sort(missing)
	return fmt.Errorf("%w: %v are blank", ErrResourceIncomplete, missing)
}

// resource puts the OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES of the
// environment over the declared attributes, which is why the identity is only
// checked on what it returns.
func (c Config) resource() (*sdkresource.Resource, error) {
	return sdkresource.New(context.Background(),
		sdkresource.WithSchemaURL(semconv.SchemaURL),
		sdkresource.WithTelemetrySDK(),
		sdkresource.WithProcessRuntimeName(),
		sdkresource.WithProcessRuntimeVersion(),
		sdkresource.WithAttributes(c.ResourceAttributes()...),
		sdkresource.WithFromEnv(),
	)
}

// EffectiveSampling is the rate table of the head; undeclared, every class is
// at 1.0, the default of traceidratio. TRC-13 lives in the tail only: at both
// ends it would multiply the rates and drop errors before the outcome is known.
func (c Config) EffectiveSampling() tracing.Rates {
	if len(c.Sampling) == 0 {
		return tracing.UniformRates(1)
	}
	return c.Sampling
}

// ResourceAttributes is the identity of the process. The sheets stay out of it:
// their effective values go on the resilience sheet in effect record (RES-40).
func (c Config) ResourceAttributes() []attribute.KeyValue {
	attributes := make([]attribute.KeyValue, 0, 4+len(c.Resource.Attributes))
	for key, value := range map[attribute.Key]string{
		semconv.ServiceNameKey:       c.Resource.ServiceName,
		semconv.ServiceVersionKey:    c.Resource.ServiceVersion,
		semconv.ServiceInstanceIDKey: c.Resource.ServiceInstanceID,
		ProcessRoleAttribute:         c.Resource.Role,
	} {
		if value != "" {
			attributes = append(attributes, key.String(value))
		}
	}
	return append(attributes, c.Resource.Attributes...)
}
