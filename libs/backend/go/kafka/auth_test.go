package kafka_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

func loggedAuth(t *testing.T, auth kafka.ClientAuth) (map[string]any, string) {
	t.Helper()
	var out bytes.Buffer
	slog.New(slog.NewJSONHandler(&out, nil)).Info("process configured", slog.Any("kafka", auth))
	var record struct {
		Kafka map[string]any `json:"kafka"`
	}
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("Unmarshal(%s) = %v", out.String(), err)
	}
	return record.Kafka, out.String()
}

func TestTheClientAuthLogsNoSecretNorPrincipal(t *testing.T) {
	auth := kafka.ReadClientAuth(func(variable string) string {
		return map[string]string{
			"KAFKA_SASL_MECHANISM":   kafka.ScramSHA512,
			"KAFKA_SASL_USERNAME":    "sentinel-principal",
			"KAFKA_SASL_PASSWORD":    "sentinel-sasl-password",
			"KAFKA_CLIENT_CERT_FILE": "/etc/kafka/client.crt",
			"KAFKA_CLIENT_KEY_FILE":  "/etc/kafka/sentinel-private.key",
			"KAFKA_CA_FILE":          "/etc/kafka/ca.crt",
		}[variable]
	})

	logged, raw := loggedAuth(t, auth)

	for _, secret := range []string{"sentinel-principal", "sentinel-sasl-password", "sentinel-private.key"} {
		if strings.Contains(raw, secret) {
			t.Errorf("record %s carries %q (LOG-09)", raw, secret)
		}
	}
	want := map[string]any{
		"sasl_mechanism": kafka.ScramSHA512,
		"sasl_username":  "set",
		"sasl_password":  redact.Placeholder,
		"cert_file":      "/etc/kafka/client.crt",
		"key_file":       "set",
		"ca_file":        "/etc/kafka/ca.crt",
	}
	if len(logged) != len(want) {
		t.Fatalf("logged %v, want %v", logged, want)
	}
	for field, value := range want {
		if logged[field] != value {
			t.Errorf("%s = %v, want %v", field, logged[field], value)
		}
	}
}

func TestAnUndeclaredClientAuthLogsEveryPositionUnset(t *testing.T) {
	logged, _ := loggedAuth(t, kafka.ReadClientAuth(func(string) string { return "" }))

	want := map[string]any{
		"sasl_mechanism": "unset",
		"sasl_username":  "unset",
		"sasl_password":  "unset",
		"cert_file":      "unset",
		"key_file":       "unset",
		"ca_file":        "unset",
	}
	for field, value := range want {
		if logged[field] != value {
			t.Errorf("%s = %v, want %v", field, logged[field], value)
		}
	}
}

func TestClientAuthMissingRequiresAPrincipalUnderTLS(t *testing.T) {
	sasl := &kafka.SASL{Mechanism: kafka.ScramSHA256, Username: "u", Password: "p"}
	for name, tc := range map[string]struct {
		auth     kafka.ClientAuth
		insecure bool
		want     []string
	}{
		"nothing under TLS":     {kafka.ClientAuth{}, false, []string{"KAFKA_SASL_MECHANISM or KAFKA_CLIENT_CERT_FILE"}},
		"SASL under TLS":        {kafka.ClientAuth{SASL: sasl}, false, nil},
		"certificate under TLS": {kafka.ClientAuth{CertFile: "/tls/client.pem"}, false, nil},
		"insecure opt-out":      {kafka.ClientAuth{}, true, nil},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.auth.Missing(tc.insecure); !slices.Equal(got, tc.want) {
				t.Fatalf("Missing(%v) = %q, want %q", tc.insecure, got, tc.want)
			}
		})
	}
}
