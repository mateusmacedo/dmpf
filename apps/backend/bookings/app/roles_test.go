package app

import (
	"reflect"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
)

func TestEachRoleIsHandedEverySettingOfConfig(t *testing.T) {
	cfg := Defaults(RoleAPI)
	cfg.DSN, cfg.Migrate, cfg.MetricTenants = "postgres://bookings@db/bookings", true, []string{"acme"}
	cfg.Brokers, cfg.KafkaInsecure, cfg.KafkaAuth = []string{"kafka:9092"}, true, kafka.ClientAuth{CAFile: "ca.pem"}

	for _, role := range []any{apiRole(cfg), relayRole(cfg)} {
		v := reflect.ValueOf(role)
		for i := range v.NumField() {
			if v.Field(i).IsZero() {
				t.Errorf("%s.%s is zero, want it set from Config", v.Type().Name(), v.Type().Field(i).Name)
			}
		}
	}
}
