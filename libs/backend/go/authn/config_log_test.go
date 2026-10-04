package authn_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	lognoop "go.opentelemetry.io/otel/log/noop"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
)

func TestTheConfigLogsHowTheEdgeResolvesIdentity(t *testing.T) {
	cfg, err := authn.ReadEnv(env(keycloakEnv()))
	if err != nil {
		t.Fatalf("ReadEnv() = %v", err)
	}
	cfg.LoggerProvider = lognoop.NewLoggerProvider()
	var out bytes.Buffer

	slog.New(slog.NewJSONHandler(&out, nil)).Info("process configured", slog.Any("authn", cfg))

	var record struct {
		Authn map[string]any `json:"authn"`
	}
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("Unmarshal(%s) = %v", out.String(), err)
	}
	want := map[string]any{
		"issuer":            cfg.Issuer,
		"audience":          cfg.Audience,
		"tenant_claim":      cfg.TenantClaim,
		"permission_claims": "scope,realm_access.roles",
		"discovery_timeout": "10s",
		"dev_mock":          false,
	}
	if len(record.Authn) != len(want) {
		t.Fatalf("logged %v, want %v and not the logger", record.Authn, want)
	}
	for field, value := range want {
		if record.Authn[field] != value {
			t.Errorf("%s = %v, want %v", field, record.Authn[field], value)
		}
	}
}
