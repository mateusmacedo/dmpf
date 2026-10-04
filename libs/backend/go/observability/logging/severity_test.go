package logging_test

import (
	"log/slog"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestSeverityFollowsTheRoleAndTheOutcome(t *testing.T) {
	for _, tc := range []struct {
		name    string
		role    logging.Role
		outcome ports.OutcomeCategory
		want    slog.Level
	}{
		{"server accepted", logging.Server, ports.OutcomeAccepted, slog.LevelInfo},
		{"server rejected", logging.Server, ports.OutcomeRejected, slog.LevelInfo},
		{"server denied", logging.Server, ports.OutcomeDenied, slog.LevelWarn},
		{"server failed", logging.Server, ports.OutcomeFailed, slog.LevelError},
		{"consumer accepted", logging.Consumer, ports.OutcomeAccepted, slog.LevelInfo},
		{"consumer rejected", logging.Consumer, ports.OutcomeRejected, slog.LevelInfo},
		{"consumer denied", logging.Consumer, ports.OutcomeDenied, slog.LevelWarn},
		{"consumer failed", logging.Consumer, ports.OutcomeFailed, slog.LevelError},
		{"client accepted", logging.Client, ports.OutcomeAccepted, slog.LevelDebug},
		{"client rejected", logging.Client, ports.OutcomeRejected, slog.LevelDebug},
		{"client denied", logging.Client, ports.OutcomeDenied, slog.LevelWarn},
		{"client failed", logging.Client, ports.OutcomeFailed, slog.LevelWarn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := logging.Severity(tc.role, tc.outcome); got != tc.want {
				t.Errorf("Severity(%v, %q) = %v, want %v", tc.role, tc.outcome, got, tc.want)
			}
		})
	}
}
