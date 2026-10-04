package authn_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const authnScope = "github.com/mateusmacedo/dmpf/libs/backend/go/authn"

type records struct {
	mu   sync.Mutex
	seen []sdklog.Record
}

func (r *records) Export(_ context.Context, batch []sdklog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range batch {
		r.seen = append(r.seen, record.Clone())
	}
	return nil
}
func (r *records) Shutdown(context.Context) error   { return nil }
func (r *records) ForceFlush(context.Context) error { return nil }

func (r *records) all() []sdklog.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]sdklog.Record(nil), r.seen...)
}

func (r *records) provider() *sdklog.LoggerProvider {
	return sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(r)))
}

func (r *records) apiProvider(raw *records) *sdklog.LoggerProvider {
	return sdklog.NewLoggerProvider(
		sdklog.WithResource(sdkresource.NewSchemaless(attribute.String(otelboot.ProcessRoleAttribute, "api"))),
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(raw)),
		sdklog.WithProcessor(otelboot.NewLogProcessor(sdklog.NewSimpleProcessor(r), otelboot.LogPolicy{})),
	)
}

func attributesOf(record sdklog.Record) map[string]string {
	got := map[string]string{}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		got[string(kv.Key)] = kv.Value.String()
		return true
	})
	return got
}

func TestEachRefusalLogsUnauthenticatedWithItsReasonPastTheAllowlist(t *testing.T) {
	provider := newIdP(t)
	other := newIdP(t)
	sink := &records{}
	raw := &records{}
	cfg := authn.Defaults()
	cfg.Issuer = provider.issuer
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "tenant_id"
	cfg.LoggerProvider = sink.apiProvider(raw)
	verifier, err := authn.NewVerifier(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewVerifier() err = %v", err)
	}

	tests := []struct {
		name   string
		reason string
		scheme string
		token  func() string
	}{
		{"token_expired", "token_expired", "Bearer", func() string {
			return provider.keycloakToken(t, map[string]any{"exp": time.Now().Add(-time.Hour).Unix()})
		}},
		{"audience_mismatch", "audience_mismatch", "Bearer", func() string { return provider.keycloakToken(t, map[string]any{"aud": "another-api"}) }},
		{"issuer_mismatch", "issuer_mismatch", "Bearer", func() string { return provider.keycloakToken(t, map[string]any{"iss": other.issuer}) }},
		{"signature_invalid", "signature_invalid", "Bearer", func() string { return other.keycloakToken(t, map[string]any{"iss": provider.issuer}) }},
		{"signature_invalid_of_an_expired_token", "signature_invalid", "Bearer", func() string {
			return other.keycloakToken(t, map[string]any{"iss": provider.issuer, "exp": time.Now().Add(-time.Hour).Unix()})
		}},
		{"signature_invalid_of_another_audience", "signature_invalid", "Bearer", func() string {
			return other.keycloakToken(t, map[string]any{"iss": provider.issuer, "aud": "another-api"})
		}},
		{"malformed", "malformed", "Bearer", func() string { return "not-a-jwt" }},
		{"verified_without_subject", "malformed", "Bearer", func() string { return provider.keycloakToken(t, map[string]any{"sub": nil}) }},
		{"not_yet_valid", "malformed", "Bearer", func() string {
			return provider.keycloakToken(t, map[string]any{"nbf": time.Now().Add(time.Hour).Unix()})
		}},
		{"token_scheme", "malformed", "Token", func() string { return provider.keycloakToken(t, nil) }},
		{"basic_scheme", "malformed", "Basic", func() string { return "YWxpY2U6c2VjcmV0" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, beforeRaw := len(sink.all()), len(raw.all())
			token := tt.token()
			_, _ = verifier.Authenticate(t.Context(), ports.Credential{Scheme: tt.scheme, Value: token})

			logged, emitted := sink.all()[before:], raw.all()[beforeRaw:]
			if len(logged) != 1 || len(emitted) != 1 {
				t.Fatalf("%d records past the processor and %d emitted, want one refusal each", len(logged), len(emitted))
			}
			record := logged[0]
			if body := record.Body().AsString(); body != "auth: token rejected" {
				t.Errorf("body = %q, want the refusal", body)
			}
			if scope := record.InstrumentationScope().Name; scope != authnScope {
				t.Errorf("scope = %q, want the import path of authn %q (RF-A1)", scope, authnScope)
			}
			if record.Severity() != log.SeverityWarn {
				t.Errorf("severity = %v, want WARN (RF-A6)", record.Severity())
			}
			got := attributesOf(record)
			if got["error.type"] != "Unauthenticated" {
				t.Errorf("error.type = %q, want the FND-07 category Unauthenticated (RF-B1)", got["error.type"])
			}
			if got["dmpf.auth.refusal_reason"] != tt.reason {
				t.Errorf("dmpf.auth.refusal_reason = %q, want %q surviving the allowlist of the api role (RF-A6)",
					got["dmpf.auth.refusal_reason"], tt.reason)
			}
			for key, value := range attributesOf(emitted[0]) {
				if strings.Contains(value, token) || strings.Contains(value, "oidc:") {
					t.Errorf("emitted attribute %s = %q carries the token or the library message", key, value)
				}
			}
		})
	}
}

func TestAKeySetTheIdPCannotServeIsLoggedAsTheDependencyNotAsTheToken(t *testing.T) {
	provider := newIdP(t)
	provider.keysDown.Store(true)
	sink := &records{}
	cfg := authn.Defaults()
	cfg.Issuer = provider.issuer
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "tenant_id"
	cfg.LoggerProvider = sink.apiProvider(&records{})
	verifier, err := authn.NewVerifier(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewVerifier() err = %v", err)
	}

	_, err = verifier.Authenticate(t.Context(), bearer(provider.keycloakToken(t, nil)))

	if !errors.Is(err, ports.ErrCredentialRejected) {
		t.Fatalf("Authenticate() err = %v, want ErrCredentialRejected", err)
	}
	logged := sink.all()
	if len(logged) != 1 {
		t.Fatalf("%d records, want one", len(logged))
	}
	got := attributesOf(logged[0])
	if reason, refused := got["dmpf.auth.refusal_reason"]; refused || got["error.type"] != "TransientDependency" {
		t.Fatalf("%q error.type = %q refusal_reason = %q, want TransientDependency and no refusal: the IdP failed, not the token",
			logged[0].Body().AsString(), got["error.type"], reason)
	}
	if logged[0].Severity() != log.SeverityWarn {
		t.Errorf("severity = %v, want WARN", logged[0].Severity())
	}
}

func TestAnAcceptedTokenLogsNoRefusal(t *testing.T) {
	provider := newIdP(t)
	sink := &records{}
	cfg := authn.Defaults()
	cfg.Issuer = provider.issuer
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "tenant_id"
	cfg.LoggerProvider = sink.provider()
	verifier, err := authn.NewVerifier(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := verifier.Authenticate(t.Context(), bearer(provider.keycloakToken(t, nil))); err != nil {
		t.Fatal(err)
	}
	if logged := sink.all(); len(logged) != 0 {
		t.Fatalf("records = %v, want none for an accepted token", logged)
	}
}

func TestAnAbsentCredentialLogsNoRefusal(t *testing.T) {
	provider := newIdP(t)
	sink := &records{}
	cfg := authn.Defaults()
	cfg.Issuer = provider.issuer
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "tenant_id"
	cfg.LoggerProvider = sink.provider()
	verifier, err := authn.NewVerifier(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		credential ports.Credential
	}{
		{name: "nothing presented", credential: ports.Credential{}},
		{name: "the bearer scheme without a value", credential: ports.Credential{Scheme: "Bearer"}},
		{name: "another scheme without a value", credential: ports.Credential{Scheme: "Basic"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := len(sink.all())
			if _, err := verifier.Authenticate(t.Context(), tc.credential); !errors.Is(err, ports.ErrCredentialAbsent) {
				t.Fatalf("Authenticate() err = %v, want ErrCredentialAbsent", err)
			}
			if logged := sink.all()[before:]; len(logged) != 0 {
				t.Errorf("%d records, want no refusal for an absent credential (RF-A6)", len(logged))
			}
		})
	}
}

func TestTheBearerSchemeIsAcceptedInAnyCase(t *testing.T) {
	provider := newIdP(t)
	sink := &records{}
	cfg := authn.Defaults()
	cfg.Issuer = provider.issuer
	cfg.Audience = "dmpf-bff"
	cfg.TenantClaim = "tenant_id"
	cfg.LoggerProvider = sink.provider()
	verifier, err := authn.NewVerifier(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	for _, scheme := range []string{"bearer", "BEARER", "BeArEr"} {
		t.Run(scheme, func(t *testing.T) {
			before := len(sink.all())
			got, err := verifier.Authenticate(t.Context(), ports.Credential{Scheme: scheme, Value: provider.keycloakToken(t, nil)})
			if err != nil {
				t.Fatalf("Authenticate() err = %v, want the scheme compared without case (RFC 7235 section 2.1, RF-A6)", err)
			}
			if got.Subject != "f:0e1a:alice" {
				t.Errorf("Subject = %q, want the subject of the token", got.Subject)
			}
			if logged := sink.all()[before:]; len(logged) != 0 {
				t.Errorf("%d records for the scheme %q, want no refusal (RF-A6)", len(logged), scheme)
			}
		})
	}
}

func TestWithoutAProviderTheRefusalGoesToTheGlobalOneOfOpenTelemetry(t *testing.T) {
	sink := &records{}
	previous := otel.GetLoggerProvider()
	otel.SetLoggerProvider(sink.provider())
	t.Cleanup(func() { otel.SetLoggerProvider(previous) })
	var stray bytes.Buffer
	previousDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&stray, nil)))
	t.Cleanup(func() { slog.SetDefault(previousDefault) })

	provider := newIdP(t)
	verifier := verifierFor(t, provider)
	_, _ = verifier.Authenticate(t.Context(), bearer("not-a-jwt"))

	if stray.Len() != 0 {
		t.Errorf("slog.Default received %q, want nothing outside OpenTelemetry (RF-A1)", stray.String())
	}
	var scopes []string
	for _, record := range sink.all() {
		scopes = append(scopes, record.InstrumentationScope().Name)
	}
	if len(scopes) != 1 || scopes[0] != authnScope {
		t.Fatalf("global provider received records under %q, want the one refusal under %q and nothing through slog.Default (RF-A1)", scopes, authnScope)
	}
}

func TestDiscoveryAndTheKeySetLeaveAClientSpanEach(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(previous) })

	provider := newIdP(t)
	verifier := verifierFor(t, provider)
	if _, err := verifier.Authenticate(t.Context(), bearer(provider.keycloakToken(t, nil))); err != nil {
		t.Fatal(err)
	}

	paths := map[string]bool{}
	for _, span := range recorder.Ended() {
		if span.SpanKind() != trace.SpanKindClient {
			continue
		}
		if span.Name() != http.MethodGet {
			t.Errorf("span name = %q, want %q (RF-B1)", span.Name(), http.MethodGet)
		}
		for _, kv := range span.Attributes() {
			if kv.Key == "url.full" {
				paths[strings.TrimPrefix(kv.Value.AsString(), provider.issuer)] = true
			}
		}
	}
	for _, want := range []string{"/.well-known/openid-configuration", "/jwks"} {
		if !paths[want] {
			t.Errorf("no CLIENT span for %s among %v (RF-B5)", want, paths)
		}
	}
}

var semconvDurationBoundaries = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func TestDiscoveryAndTheKeySetRecordTheClientRequestDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(previous) })

	provider := newIdP(t)
	verifier := verifierFor(t, provider)
	if _, err := verifier.Authenticate(t.Context(), bearer(provider.keycloakToken(t, nil))); err != nil {
		t.Fatal(err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	var duration *metricdata.Metrics
	for _, scope := range rm.ScopeMetrics {
		for i := range scope.Metrics {
			if scope.Metrics[i].Name == "http.client.request.duration" {
				duration = &scope.Metrics[i]
			}
		}
	}
	if duration == nil {
		t.Fatal("no http.client.request.duration series (RF-D2)")
	}
	if duration.Unit != "s" {
		t.Errorf("unit = %q, want s", duration.Unit)
	}
	histogram, ok := duration.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("aggregation = %T, want a float64 histogram", duration.Data)
	}
	var requests uint64
	for _, point := range histogram.DataPoints {
		requests += point.Count
		if method, _ := point.Attributes.Value("http.request.method"); method.AsString() != http.MethodGet {
			t.Errorf("http.request.method = %q, want GET", method.AsString())
		}
		if status, _ := point.Attributes.Value("http.response.status_code"); status.AsInt64() != http.StatusOK {
			t.Errorf("http.response.status_code = %d, want 200", status.AsInt64())
		}
		if !slices.Equal(point.Bounds, semconvDurationBoundaries) {
			t.Errorf("bounds = %v, want the semconv advisory %v (RF-D4)", point.Bounds, semconvDurationBoundaries)
		}
	}
	if requests != 2 {
		t.Fatalf("%d requests measured, want the discovery and the key set (RF-B5)", requests)
	}
}
