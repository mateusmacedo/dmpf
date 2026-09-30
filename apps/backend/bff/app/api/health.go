package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	LivenessPath  = "/livez"
	ReadinessPath = "/readyz"

	readinessBudget = 2 * time.Second
	readinessTTL    = time.Second
)

func serveLiveness() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

type readiness struct {
	check    func(context.Context) error
	draining func() bool
	logger   *slog.Logger
	mu       sync.Mutex
	checked  time.Time
	err      error
}

func serveReadiness(check func(context.Context) error, draining func() bool, logger *slog.Logger) http.Handler {
	return &readiness{check: check, draining: draining, logger: logger}
}

func (h *readiness) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if (h.draining != nil && h.draining()) || h.result(r.Context()) != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *readiness) result(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.checked.IsZero() && time.Since(h.checked) < readinessTTL {
		return h.err
	}
	probe, cancel := context.WithTimeout(context.WithoutCancel(ctx), readinessBudget)
	defer cancel()
	h.err, h.checked = h.check(probe), time.Now()
	if h.err != nil {
		h.logger.WarnContext(ctx, "not ready", "error", h.err.Error())
	}
	return h.err
}
