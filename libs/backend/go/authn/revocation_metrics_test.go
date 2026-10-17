package authn_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func withMeter(reader *sdkmetric.ManualReader) func(*authn.Config) {
	return func(cfg *authn.Config) {
		cfg.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	}
}

func revocationCounters(t *testing.T, reader *sdkmetric.ManualReader) map[string]map[string]int64 {
	t.Helper()

	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	got := map[string]map[string]int64{}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if !strings.HasPrefix(m.Name, "dmpf.auth.revocation.") {
				continue
			}
			if scope.Scope.Name != authnScope {
				t.Errorf("%s under scope %q, want %q", m.Name, scope.Scope.Name, authnScope)
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok || !sum.IsMonotonic {
				t.Fatalf("%s = %T, want a monotonic int64 counter", m.Name, m.Data)
			}
			points := map[string]int64{}
			for _, point := range sum.DataPoints {
				var labels []string
				for _, kv := range point.Attributes.ToSlice() {
					labels = append(labels, string(kv.Key)+"="+kv.Value.String())
				}
				points[strings.Join(labels, ",")] += point.Value
			}
			got[m.Name] = points
		}
	}
	return got
}

func TestTheRefusalsCountTheKindTheDenyListAnswered(t *testing.T) {
	issued := time.Now().Add(-time.Minute).Truncate(time.Second)

	tests := []struct {
		name    string
		entries []authn.Revocation
		want    string
	}{
		{"session", []authn.Revocation{revokedSession(aliceSession)}, "kind=session"},
		{"subject", []authn.Revocation{revokedSubject(issued)}, "kind=subject"},
		{"both, session first", []authn.Revocation{revokedSubject(issued), revokedSession(aliceSession)}, "kind=session"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := sdkmetric.NewManualReader()
			f := newRevocationFixture(t, 0, withMeter(reader))
			for _, entry := range tt.entries {
				f.revoke(t, entry)
			}

			_, _ = f.verifier.Authenticate(t.Context(), bearer(f.provider.keycloakToken(t,
				map[string]any{"sid": aliceSession, "iat": issued.Unix()})))

			got := revocationCounters(t, reader)
			refusals := got["dmpf.auth.revocation.refusals"]
			if len(refusals) != 1 || refusals[tt.want] != 1 {
				t.Fatalf("dmpf.auth.revocation.refusals = %v, want one refusal with %s and nothing else", refusals, tt.want)
			}
			if errs := got["dmpf.auth.revocation.store_errors"]; len(errs) != 0 {
				t.Errorf("dmpf.auth.revocation.store_errors = %v, want none", errs)
			}
		})
	}
}

func TestAStoreErrorCountsWithoutAttributesAndWithoutARefusal(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	f := newRevocationFixture(t, 0, withMeter(reader))
	f.checker.failWith(errors.New("store: down"))

	_, _ = f.verifier.Authenticate(t.Context(), bearer(f.provider.keycloakToken(t, map[string]any{"sid": aliceSession})))

	got := revocationCounters(t, reader)
	if errs := got["dmpf.auth.revocation.store_errors"]; len(errs) != 1 || errs[""] != 1 {
		t.Fatalf("dmpf.auth.revocation.store_errors = %v, want one error and no attribute", errs)
	}
	if refusals := got["dmpf.auth.revocation.refusals"]; len(refusals) != 0 {
		t.Errorf("dmpf.auth.revocation.refusals = %v, want none: the store failed, not the token", refusals)
	}
}

func TestAnAcceptedTokenCountsNothing(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	f := newRevocationFixture(t, 0, withMeter(reader))

	if _, err := f.verifier.Authenticate(t.Context(), bearer(f.provider.keycloakToken(t, nil))); err != nil {
		t.Fatal(err)
	}

	for name, points := range revocationCounters(t, reader) {
		if len(points) != 0 {
			t.Errorf("%s = %v, want nothing for an accepted token", name, points)
		}
	}
}

func TestWithoutAMeterProviderTheRevocationMetricsGoToTheGlobalOne(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(previous) })

	f := newRevocationFixture(t, 0)
	f.revoke(t, revokedSession(aliceSession))
	_, _ = f.verifier.Authenticate(t.Context(), bearer(f.provider.keycloakToken(t, map[string]any{"sid": aliceSession})))

	if refusals := revocationCounters(t, reader)["dmpf.auth.revocation.refusals"]; refusals["kind=session"] != 1 {
		t.Fatalf("global dmpf.auth.revocation.refusals = %v, want the refusal counted", refusals)
	}
}

type answeringChecker struct {
	kind   authn.RevocationKind
	err    error
	before func()
}

func (c answeringChecker) Revoked(context.Context, authn.SessionRef) (authn.RevocationKind, error) {
	if c.before != nil {
		c.before()
	}
	return c.kind, c.err
}

func TestARefusalKindOutsideTheContractCountsAsSubject(t *testing.T) {
	tests := []struct {
		name string
		kind authn.RevocationKind
		want string
	}{
		{"session", authn.RevocationSession, "kind=session"},
		{"subject", authn.RevocationSubject, "kind=subject"},
		{"echoes the sid", authn.RevocationKind("session:" + aliceSession), "kind=subject"},
		{"arbitrary", authn.RevocationKind("tenant"), "kind=subject"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := sdkmetric.NewManualReader()
			f := newRevocationFixture(t, 0, withMeter(reader), func(cfg *authn.Config) {
				cfg.Revocations = answeringChecker{kind: tt.kind}
			})

			_, err := f.verifier.Authenticate(t.Context(), bearer(f.provider.keycloakToken(t, map[string]any{"sid": aliceSession})))

			if !errors.Is(err, ports.ErrCredentialRejected) {
				t.Fatalf("Authenticate() err = %v, want ErrCredentialRejected", err)
			}
			refusals := revocationCounters(t, reader)["dmpf.auth.revocation.refusals"]
			if len(refusals) != 1 || refusals[tt.want] != 1 {
				t.Fatalf("dmpf.auth.revocation.refusals = %v, want one refusal with %s and nothing else", refusals, tt.want)
			}
		})
	}
}

func TestOnlyTheStoreFailingCountsAsAStoreError(t *testing.T) {
	tests := []struct {
		name       string
		cancelled  bool
		wantErrors int64
		wantLogged int
	}{
		{"store failed", false, 1, 1},
		{"caller cancelled", true, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := sdkmetric.NewManualReader()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			checker := answeringChecker{err: errors.New("store: down")}
			if tt.cancelled {
				checker = answeringChecker{err: context.Canceled, before: cancel}
			}
			f := newRevocationFixture(t, 0, withMeter(reader), func(cfg *authn.Config) { cfg.Revocations = checker })

			_, err := f.verifier.Authenticate(ctx, bearer(f.provider.keycloakToken(t, map[string]any{"sid": aliceSession})))

			if !errors.Is(err, ports.ErrCredentialRejected) {
				t.Fatalf("Authenticate() err = %v, want ErrCredentialRejected: the edge fails closed", err)
			}
			got := revocationCounters(t, reader)
			if errs := got["dmpf.auth.revocation.store_errors"]; errs[""] != tt.wantErrors {
				t.Errorf("dmpf.auth.revocation.store_errors = %v, want %d", errs, tt.wantErrors)
			}
			if refusals := got["dmpf.auth.revocation.refusals"]; len(refusals) != 0 {
				t.Errorf("dmpf.auth.revocation.refusals = %v, want none", refusals)
			}
			if logged := f.sink.all(); len(logged) != tt.wantLogged {
				t.Errorf("%d records, want %d", len(logged), tt.wantLogged)
			}
		})
	}
}
