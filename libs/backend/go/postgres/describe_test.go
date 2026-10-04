package postgres_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func TestTheDSNIsDescribedWithoutItsSecrets(t *testing.T) {
	t.Setenv("PGSSLMODE", "")
	cases := []struct {
		name string
		dsn  string
		want map[string]any
	}{
		{
			name: "url with password",
			dsn:  "postgres://sentinel-user:sentinel-password@db.internal:6543/orders?sslmode=verify-full",
			want: map[string]any{"host": "db.internal", "port": float64(6543), "database": "orders", "sslmode": "verify-full", "user": "set"},
		},
		{
			name: "keyword form with password",
			dsn:  "host=db.internal port=5433 dbname=reservations user=sentinel-user password=sentinel-password sslmode=disable",
			want: map[string]any{"host": "db.internal", "port": float64(5433), "database": "reservations", "sslmode": "disable", "user": "set"},
		},
		{
			name: "url without sslmode",
			dsn:  "postgresql://sentinel-user:sentinel-password@db.internal/bookings",
			want: map[string]any{"host": "db.internal", "port": float64(5432), "database": "bookings", "sslmode": "prefer", "user": "set"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			slog.New(slog.NewJSONHandler(&out, nil)).Info("process configured", slog.Any("database", postgres.DescribeDSN(tc.dsn)))

			for _, secret := range []string{"sentinel-user", "sentinel-password", tc.dsn} {
				if strings.Contains(out.String(), secret) {
					t.Errorf("record %s carries %q (LOG-09)", out.String(), secret)
				}
			}
			var record struct {
				Database map[string]any `json:"database"`
			}
			if err := json.Unmarshal(out.Bytes(), &record); err != nil {
				t.Fatalf("Unmarshal(%s) = %v", out.String(), err)
			}
			if len(record.Database) != len(tc.want) {
				t.Fatalf("logged %v, want %v", record.Database, tc.want)
			}
			for field, value := range tc.want {
				if record.Database[field] != value {
					t.Errorf("%s = %v, want %v", field, record.Database[field], value)
				}
			}
		})
	}
}

func TestAnUnparsableDSNIsDescribedWithoutItsText(t *testing.T) {
	dsn := "postgres://sentinel-user:sentinel-password@db.internal:notaport/orders"

	described := postgres.DescribeDSN(dsn)

	if described.String() != "invalid" {
		t.Fatalf("DescribeDSN() = %v, want invalid and never the text of the DSN", described)
	}
}

func TestTheDeclaredUserIsReportedAsSetOrUnset(t *testing.T) {
	cases := []struct {
		name   string
		dsn    string
		pguser string
		want   string
	}{
		{name: "url with user", dsn: "postgres://sentinel-user@db.internal:6543/orders", want: "set"},
		{name: "url without user", dsn: "postgresql://db.internal:6543/orders", want: "unset"},
		{name: "keyword form with user", dsn: "host=db.internal dbname=orders user=sentinel-user", want: "set"},
		{name: "keyword form without user", dsn: "host=db.internal dbname=orders", want: "unset"},
		{name: "PGUSER with dsn without user", dsn: "postgres://db.internal/orders", pguser: "sentinel-user", want: "set"},
		{name: "invalid dsn", dsn: "postgres://sentinel-user@db.internal:notaport/orders", want: "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PGUSER", tc.pguser)
			var out bytes.Buffer

			slog.New(slog.NewJSONHandler(&out, nil)).Info("process configured", slog.Any("database", postgres.DescribeDSN(tc.dsn)))

			if strings.Contains(out.String(), "sentinel-user") {
				t.Errorf("record %s carries the user (LOG-09)", out.String())
			}
			var record struct {
				Database json.RawMessage `json:"database"`
			}
			if err := json.Unmarshal(out.Bytes(), &record); err != nil {
				t.Fatalf("Unmarshal(%s) = %v", out.String(), err)
			}
			if tc.want == "invalid" {
				if string(record.Database) != `"invalid"` {
					t.Fatalf("database = %s, want \"invalid\"", record.Database)
				}
				return
			}
			var described map[string]any
			if err := json.Unmarshal(record.Database, &described); err != nil {
				t.Fatalf("Unmarshal(%s) = %v", record.Database, err)
			}
			if described["user"] != tc.want {
				t.Errorf("user = %v, want %s", described["user"], tc.want)
			}
		})
	}
}
