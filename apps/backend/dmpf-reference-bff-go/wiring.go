package dmpfreferencebff

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-transport/admission"

	"github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference-bff-go/api"
	"github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference-bff-go/rpc"
)

const (
	admissionKeys = 64

	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 16
	shutdownGrace     = 10 * time.Second
)

// Run boots the telemetry and serves the edge until ctx is cancelled. Every
// concrete provider of the process is built below this call (ADR-015).
func Run(ctx context.Context, cfg Config, out io.Writer) error {
	rt, err := NewTelemetry(ctx, cfg, out)
	if err != nil {
		return err
	}
	defer shutdownTelemetry(ctx, rt)

	return RunWith(ctx, cfg, rt)
}

func RunWith(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	opts, err := ClientOptions(ctx, cfg, rt)
	if err != nil {
		return err
	}
	ordersConn, err := rpc.Dial(cfg.OrdersTarget, rpc.OrdersConfig(opts))
	if err != nil {
		return err
	}
	defer func() { _ = ordersConn.Close() }()
	reservationsConn, err := rpc.Dial(cfg.ReservationsTarget, rpc.ReservationsConfig(opts))
	if err != nil {
		return err
	}
	defer func() { _ = reservationsConn.Close() }()

	ctrl, err := NewAdmission(cfg.Admission)
	if err != nil {
		return err
	}
	options := api.Options{Budget: cfg.RouteBudget, CORSOrigins: cfg.CORSOrigins}
	if options.OrdersContract, err = readContract(cfg.OrdersContractPath); err != nil {
		return err
	}
	if options.ReservationsContract, err = readContract(cfg.ReservationsContractPath); err != nil {
		return err
	}
	handler, err := api.NewHandler(rpc.NewOrders(ordersConn), rpc.NewReservations(reservationsConn), ctrl, rt.Tracer(), rt.Instruments(), options)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		// WHY: a nil ErrorLog sends net/http's own errors to the default logger
		// on stderr, around the redacting handler this process installs.
		ErrorLog: slog.NewLogLogger(rt.Logger().Handler(), slog.LevelWarn),
	}
	failed := make(chan error, 1)
	go func() { failed <- server.Serve(listener) }()
	rt.Logger().InfoContext(ctx, "http listening", "addr", listener.Addr().String(),
		"orders", cfg.OrdersTarget, "reservations", cfg.ReservationsTarget)

	select {
	case <-ctx.Done():
		grace, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
		defer cancel()
		return server.Shutdown(grace)
	case err := <-failed:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// ClientOptions resolves the transport policy of both clients: TLS through the
// declared authority, or the development opt-out, logged so an audit finds it.
func ClientOptions(ctx context.Context, cfg Config, rt *otelboot.Runtime) (rpc.Options, error) {
	opts := rpc.Options{
		Clock:       obsclock.System(),
		Tracer:      rt.Tracer(),
		Instruments: rt.Instruments(),
		Logger:      rt.Logger(),
		Service:     cfg.Service,
	}
	if cfg.CAFile == "" {
		// WHY: FromEnv guarantees the pair, but Run, RunWith and this function
		// are exported and a caller that builds Config by hand would otherwise
		// dial in the clear from a single empty field.
		if !cfg.GRPCInsecure {
			return rpc.Options{}, ErrInsecureNotDeclared
		}
		rt.Logger().WarnContext(ctx, "grpc clients without TLS: DMPF_GRPC_INSECURE is set (development and CI only)")
		opts.Insecure = true
		return opts, nil
	}
	clientTLS, err := rpc.ClientTLS(cfg.CAFile, cfg.ServerName)
	if err != nil {
		return rpc.Options{}, err
	}
	opts.TLS = clientTLS
	return opts, nil
}

func NewAdmission(limit admission.Limit) (*admission.Controller, error) {
	tenants, err := metrics.DeclareTenants(api.Tenant)
	if err != nil {
		return nil, err
	}
	return admission.New(admission.Config{
		Limits:  api.Limits(limit),
		Tenants: tenants,
		MaxKeys: admissionKeys,
		Clock:   obsclock.System(),
	})
}

func readContract(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	document, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("openapi: %w", err)
	}
	return document, nil
}

// WHY: context.WithoutCancel drops the cancellation and adds no deadline, and
// otelboot.Runtime.Shutdown hands the context straight to both providers, so an
// unreachable collector blocks the exit until the orchestrator sends SIGKILL.
func shutdownTelemetry(ctx context.Context, rt *otelboot.Runtime) {
	grace, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
	defer cancel()
	if err := rt.Shutdown(grace); err != nil {
		rt.Logger().WarnContext(grace, "telemetry shutdown", "error", err.Error())
	}
}
