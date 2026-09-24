// Package envconfig reads the values a process declares in its environment.
//
// Configuration enters only by environment variable and is validated at
// startup, so every parser here answers with an error the caller turns into
// exit 2 instead of degrading to a default it did not declare.
package envconfig

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ErrInvalidVariable is the sentinel every parser wraps, so a caller
// recognises an invalid declaration with errors.Is without matching text.
var ErrInvalidVariable = errors.New("config: variable has an invalid value")

func OrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// Hostname answers "local" when the operating system has no name to give: the
// value names the instance in telemetry and must never fail the startup.
func Hostname() string {
	if name, err := os.Hostname(); err == nil && name != "" {
		return name
	}
	return "local"
}

func SplitList(value string) []string {
	var items []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func ParseBool(variable, value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%w: %s=%q is not a boolean", ErrInvalidVariable, variable, value)
	}
	return parsed, nil
}

func ParsePositive(variable, value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%w: %s=%q is not a positive integer", ErrInvalidVariable, variable, value)
	}
	return parsed, nil
}
