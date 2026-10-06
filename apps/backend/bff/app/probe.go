package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/api"
)

var ErrNotReady = errors.New("bff: not ready")

const probeBodyLimit = 512

// Probe asks the edge listening on cfg.AdminAddr for its readiness: the check an
// image without a shell runs against itself.
func Probe(ctx context.Context, cfg Config) error {
	url, err := readinessURL(cfg.AdminAddr)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotReady, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, probeBodyLimit))
	return fmt.Errorf("%w: %s %s", ErrNotReady, resp.Status, strings.TrimSpace(string(body)))
}

func readinessURL(listen string) (string, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", fmt.Errorf("probe: %w", err)
	}
	if ip := net.ParseIP(host); host == "" || ip != nil && ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + api.ReadinessPath, nil
}
