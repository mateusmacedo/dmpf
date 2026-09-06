package otelboot

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
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

	// ErrInsecureNotAllowed is a transport that turns TLS off without saying so
	// in the configuration. Plaintext telemetry is a choice, not a fallback.
	ErrInsecureNotAllowed = errors.New("otelboot: an insecure transport requires AllowInsecure")
)

// The W3C Trace Context headers a conforming propagator injects. The check is
// on the fields the propagator declares, not on its type, so a composite that
// adds baggage still passes.
const (
	headerTraceparent = "traceparent"
	headerTracestate  = "tracestate"
)

// SheetAttributePrefix opens the resource attribute of a resilience sheet. The
// full key is <prefix><dependency>.<field> (RES-40).
const SheetAttributePrefix = "dmpf.sheet."

// Transport is how the exporters reach the collector. TLS is the default and
// Insecure turns it off, which is why turning it off takes two declarations
// (this one and Config.AllowInsecure).
type Transport struct {
	Endpoint string
	Insecure bool
	TLS      *tls.Config
}

// Resource is the identity of the process behind the telemetry. The three
// service fields are mandatory because a signal nobody can attribute to a
// service, a version and an instance is a signal nobody can act on.
type Resource struct {
	ServiceName       string
	ServiceVersion    string
	ServiceInstanceID string

	// Attributes are further resource attributes, such as the deployment
	// environment, that the caller adds on its own terms.
	Attributes []attribute.KeyValue
}

// Config is everything the bootstrap needs. The exporters are injected so the
// suite runs the whole pipeline in memory, and otelboot/otlp supplies the
// production ones from Transport.
type Config struct {
	Propagator    propagation.TextMapPropagator
	Resource      Resource
	Sampling      tracing.Rates
	Transport     Transport
	AllowInsecure bool
	Sheets        []resilience.Sheet
	TraceExporter sdktrace.SpanExporter
	MetricReader  sdkmetric.Reader

	// Logger records the start-up of the runtime. It is optional: a nil logger
	// discards those records rather than inventing a destination.
	Logger *slog.Logger
}

// Validate reports every fault at once, so a reader fixes the configuration in
// one pass instead of discovering the next fault on the next run.
func (c Config) Validate() error {
	faults := []error{c.validatePropagator(), c.validateResource(), c.ValidateTransport()}
	for _, sheet := range c.Sheets {
		faults = append(faults, sheet.Validate())
	}
	return errors.Join(faults...)
}

func (c Config) validatePropagator() error {
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
	missing := make([]string, 0, 3)
	for key, value := range map[attribute.Key]string{
		semconv.ServiceNameKey:       c.Resource.ServiceName,
		semconv.ServiceVersionKey:    c.Resource.ServiceVersion,
		semconv.ServiceInstanceIDKey: c.Resource.ServiceInstanceID,
	} {
		if value == "" {
			missing = append(missing, string(key))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	slices.Sort(missing)
	return fmt.Errorf("%w: %v are blank", ErrResourceIncomplete, missing)
}

// ValidateTransport is the transport rule on its own, so the package that
// builds the OTLP exporters applies the same one instead of restating it.
func (c Config) ValidateTransport() error {
	if c.Transport.Insecure && !c.AllowInsecure {
		return fmt.Errorf("%w: %s would carry telemetry in plaintext", ErrInsecureNotAllowed, c.Transport.Endpoint)
	}
	return nil
}

// EffectiveSampling is the rate table in use. An undeclared table takes the
// platform baseline of TRC-13 rather than resolving every class to the most
// restrictive rate, which would silently stop sampling errors.
func (c Config) EffectiveSampling() tracing.Rates {
	if len(c.Sampling) == 0 {
		return tracing.DefaultRates()
	}
	return c.Sampling
}

// ResourceAttributes is the identity of the process plus the effective values
// of every sheet. The resource takes no attributes after it is created, so the
// sheets are an input of the bootstrap and not something registered later
// (RES-40).
func (c Config) ResourceAttributes() []attribute.KeyValue {
	attributes := make([]attribute.KeyValue, 0, 3+len(c.Resource.Attributes)+len(c.Sheets)*10)
	attributes = append(attributes,
		semconv.ServiceName(c.Resource.ServiceName),
		semconv.ServiceVersion(c.Resource.ServiceVersion),
		semconv.ServiceInstanceID(c.Resource.ServiceInstanceID),
	)
	attributes = append(attributes, c.Resource.Attributes...)

	for _, sheet := range c.Sheets {
		for field, value := range sheet.Effective() {
			key := SheetAttributePrefix + sheet.Dependency + "." + field
			attributes = append(attributes, attribute.String(key, value))
		}
	}
	return attributes
}
