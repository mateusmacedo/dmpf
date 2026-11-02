package s3

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const testKeyARN = "arn:aws:kms:us-east-1:000000000000:key/00000000-0000-0000-0000-000000000000"

func validConfig() Config {
	return Config{
		AWS:    aws.Config{Region: "us-east-1"},
		Bucket: "evidence",
		Declaration: ports.ObjectStoreDeclaration{
			Scope:      ports.ScopeTenant,
			Encryption: ports.Encryption{Algorithm: "aws:kms", KeyRef: testKeyARN},
			Ceiling:    &ports.RetentionCeiling{Span: ports.RetentionSpan(400 * 24 * time.Hour), Origin: "test-ceiling"},
		},
		Clock: clock.System(),
		Sheet: resilience.Defaults("s3"),
	}
}

func TestConfigValidate(t *testing.T) {
	cases := map[string]struct {
		mutate func(*Config)
		want   error
	}{
		"complete":                      {func(*Config) {}, nil},
		"without region":                {func(c *Config) { c.AWS.Region = "" }, ErrIncompleteConfig},
		"without bucket":                {func(c *Config) { c.Bucket = "" }, ErrIncompleteConfig},
		"without clock":                 {func(c *Config) { c.Clock = nil }, ErrIncompleteConfig},
		"path style without endpoint":   {func(c *Config) { c.UsePathStyle = true }, ErrIncompleteConfig},
		"path style with endpoint":      {func(c *Config) { c.UsePathStyle = true; c.Endpoint = "https://s3.example" }, nil},
		"plaintext endpoint":            {func(c *Config) { c.Endpoint = "http://localhost:4566" }, ErrTLSRequired},
		"plaintext endpoint by opt-out": {func(c *Config) { c.Endpoint = "http://localhost:4566"; c.InsecureForDevelopmentOnly = true }, nil},
		"undeclared scope":              {func(c *Config) { c.Declaration.Scope = "" }, ports.ErrScopeUndeclared},
		"undeclared encryption":         {func(c *Config) { c.Declaration.Encryption = ports.Encryption{} }, ports.ErrEncryptionUndeclared},
		"ceiling without origin":        {func(c *Config) { c.Declaration.Ceiling.Origin = "" }, ports.ErrRetentionCeilingUndeclared},
		"store without ceiling":         {func(c *Config) { c.Declaration.Ceiling = nil }, nil},
		"sheet with a blank":            {func(c *Config) { c.Sheet.Deadline = resilience.Field[time.Duration]{} }, resilience.ErrBlankField},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if tc.want == nil && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}
