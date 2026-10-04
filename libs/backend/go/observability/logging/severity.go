package logging

import (
	"log/slog"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type Role int

const (
	Server Role = iota
	Consumer
	Client
)

func Severity(role Role, outcome ports.OutcomeCategory) slog.Level {
	if role == Client {
		switch outcome {
		case ports.OutcomeAccepted, ports.OutcomeRejected:
			return slog.LevelDebug
		default:
			return slog.LevelWarn
		}
	}

	switch outcome {
	case ports.OutcomeAccepted, ports.OutcomeRejected:
		return slog.LevelInfo
	case ports.OutcomeDenied:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}
