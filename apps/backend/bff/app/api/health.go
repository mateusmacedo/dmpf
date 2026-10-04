package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

const (
	LivenessPath  = "/livez"
	ReadinessPath = "/readyz"

	readinessBudget = 2 * time.Second
	readinessTTL    = time.Second

	healthLogInterval = 5 * time.Minute
)

func NewAdminHandler(opts Options) http.Handler {
	logger, clock := loggerOf(opts), clockOf(opts)
	mux := http.NewServeMux()
	mux.Handle("GET "+LivenessPath, withHealthLog(logger, clock, LivenessPath, serveLiveness()))
	if opts.Ready != nil {
		mux.Handle("GET "+ReadinessPath, withHealthLog(logger, clock, ReadinessPath, serveReadiness(opts.Ready, opts.Draining, logger)))
	}
	edge := append(instrumentation(opts), otelhttp.WithFilter(outsideHealth(mux)))
	return otelhttp.NewHandler(withRoute(withAccessLog(logger, mux)), "bff", edge...)
}

func outsideHealth(mux *http.ServeMux) otelhttp.Filter {
	return func(r *http.Request) bool {
		served, _ := mux.Handler(r)
		_, health := served.(*healthAccess)
		return !health
	}
}

type healthAccess struct {
	next   http.Handler
	logger *slog.Logger
	clock  obsclock.Clock
	route  string
	mu     sync.Mutex
	logged bool
	last   time.Time
}

func withHealthLog(logger *slog.Logger, clock obsclock.Clock, route string, next http.Handler) *healthAccess {
	return &healthAccess{next: next, logger: logger, clock: clock, route: route}
}

func (h *healthAccess) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	markSelfLogged(rw)
	w := &statusRecorder{ResponseWriter: rw, status: http.StatusOK}
	h.next.ServeHTTP(w, r)

	outcome := outcomeOf(w.status)
	level := logging.Severity(logging.Server, outcome)
	succeeded := w.status >= http.StatusOK && w.status < http.StatusMultipleChoices
	if !h.logger.Enabled(r.Context(), level) || (succeeded && !h.due()) {
		return
	}
	h.logger.LogAttrs(r.Context(), level, "http request", accessAttrs(r.Method, h.route, w.status, outcome)...)
}

func (h *healthAccess) due() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.clock.Now()
	if h.logged && now.Sub(h.last) < healthLogInterval {
		return false
	}
	h.logged, h.last = true, now
	return true
}

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
		h.logger.WarnContext(ctx, "not ready", redact.Error(h.err))
	}
	return h.err
}
