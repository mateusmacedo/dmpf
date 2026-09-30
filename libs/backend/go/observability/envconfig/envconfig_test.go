package envconfig_test

import (
	"errors"
	"log/slog"
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

func TestParseFractionAnswersUnsetForTheEmptyVariable(t *testing.T) {
	if _, set, err := envconfig.ParseFraction("RATE", ""); err != nil || set {
		t.Fatalf("ParseFraction(\"\") = set %v, %v; want unset, nil", set, err)
	}
}

func TestParseFractionReadsTheClosedUnitInterval(t *testing.T) {
	for raw, want := range map[string]float64{"0": 0, "0.25": 0.25, "1": 1} {
		got, set, err := envconfig.ParseFraction("RATE", raw)
		if err != nil || !set || got != want {
			t.Fatalf("ParseFraction(%q) = %v, %v, %v; want %v, true, nil", raw, got, set, err, want)
		}
	}
}

func TestParseFractionRefusesOutsideTheUnitInterval(t *testing.T) {
	for _, raw := range []string{"-0.1", "1.5", "NaN", "all"} {
		if _, _, err := envconfig.ParseFraction("RATE", raw); !errors.Is(err, envconfig.ErrInvalidVariable) {
			t.Fatalf("ParseFraction(%q) = %v, want ErrInvalidVariable", raw, err)
		}
	}
}

func TestParseLevelAnswersTheFallbackForTheEmptyVariable(t *testing.T) {
	if got, err := envconfig.ParseLevel("LEVEL", "", slog.LevelInfo); err != nil || got != slog.LevelInfo {
		t.Fatalf("ParseLevel(\"\") = %v, %v; want info, nil", got, err)
	}
}

func TestParseLevelReadsTheSlogNames(t *testing.T) {
	for raw, want := range map[string]slog.Level{"debug": slog.LevelDebug, "INFO": slog.LevelInfo, "warn": slog.LevelWarn, "error": slog.LevelError} {
		if got, err := envconfig.ParseLevel("LEVEL", raw, slog.LevelInfo); err != nil || got != want {
			t.Fatalf("ParseLevel(%q) = %v, %v; want %v", raw, got, err, want)
		}
	}
}

func TestParseLevelRefusesAnUnknownName(t *testing.T) {
	if _, err := envconfig.ParseLevel("LEVEL", "verbose", slog.LevelInfo); !errors.Is(err, envconfig.ErrInvalidVariable) {
		t.Fatalf("ParseLevel(verbose) = %v, want ErrInvalidVariable", err)
	}
}
