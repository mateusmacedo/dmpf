package envconfig_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
)

func TestOrDefaultAnswersTheFallbackOnlyForTheUnsetVariable(t *testing.T) {
	if got := envconfig.OrDefault("", "fallback"); got != "fallback" {
		t.Fatalf("OrDefault(\"\") = %q, want the fallback", got)
	}
	if got := envconfig.OrDefault("declared", "fallback"); got != "declared" {
		t.Fatalf("OrDefault(%q) = %q, want the declared value", "declared", got)
	}
}

func TestHostnameNeverAnswersEmpty(t *testing.T) {
	if envconfig.Hostname() == "" {
		t.Fatal("Hostname() = \"\"; the value names the instance in telemetry and must never fail the startup")
	}
}

func TestSplitListTrimsAndDropsTheEmptyEntries(t *testing.T) {
	got := envconfig.SplitList(" broker-1 , , broker-2 ,")

	if want := []string{"broker-1", "broker-2"}; !slices.Equal(got, want) {
		t.Fatalf("SplitList() = %q, want %q", got, want)
	}
	if got := envconfig.SplitList(""); got != nil {
		t.Fatalf("SplitList(\"\") = %q, want nil", got)
	}
}

func TestParseBoolReadsTheUnsetVariableAsFalse(t *testing.T) {
	got, err := envconfig.ParseBool("GRPC_INSECURE", "")

	if err != nil || got {
		t.Fatalf("ParseBool(\"\") = (%v, %v), want (false, nil)", got, err)
	}
}

func TestParseBoolRefusesWhatIsNotABoolean(t *testing.T) {
	_, err := envconfig.ParseBool("GRPC_INSECURE", "yes-please")

	if !errors.Is(err, envconfig.ErrInvalidVariable) {
		t.Fatalf("ParseBool() = %v, want ErrInvalidVariable", err)
	}
	if !strings.Contains(err.Error(), "GRPC_INSECURE") {
		t.Fatalf("ParseBool() = %v, want the message to name the variable", err)
	}
}

func TestParsePositiveAnswersTheFallbackForTheUnsetVariable(t *testing.T) {
	got, err := envconfig.ParsePositive("ITEM_LIMIT", "", 7)

	if err != nil || got != 7 {
		t.Fatalf("ParsePositive(\"\") = (%d, %v), want (7, nil)", got, err)
	}
}

func TestParsePositiveRefusesZeroAndBelow(t *testing.T) {
	for _, value := range []string{"0", "-1", "three"} {
		if _, err := envconfig.ParsePositive("ITEM_LIMIT", value, 7); !errors.Is(err, envconfig.ErrInvalidVariable) {
			t.Fatalf("ParsePositive(%q) = %v, want ErrInvalidVariable", value, err)
		}
	}
}
