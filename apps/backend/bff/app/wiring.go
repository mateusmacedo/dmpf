package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/api"
	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 16

	keyDrainDelay = "dmpf.drain.delay"
)

func Run(ctx context.Context, cfg Config) error {
	return boot.Boot(ctx, TelemetryOf(cfg), func(ctx context.Context, rt *otelboot.Runtime) error {
		return RunWith(ctx, cfg, rt)
	})
}

// Authenticator resolves how this process verifies identity. The development
// mock is only reachable through the opt-out the config already refused to
// combine with an issuer, so one start never has two ways of resolving it.
func Authenticator(ctx context.Context, cfg Config) (ports.Authenticator, error) {
	if cfg.Auth.DevMock {
		return authn.DevAuthenticator{}, nil
	}
	return authn.NewVerifier(ctx, cfg.Auth)
}

func RunWith(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	logger := rt.LoggerFor(reflect.TypeFor[Config]().PkgPath())
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
	bookingsConn, err := rpc.Dial(cfg.BookingsTarget, rpc.BookingsConfig(opts))
	if err != nil {
		return err
	}
	defer func() { _ = bookingsConn.Close() }()

	ctrl, err := admission.NewController(api.Limits(cfg.Admission), cfg.MetricTenants, admission.DefaultMaxKeys)
	if err != nil {
		return err
	}
	cfg.Auth.LoggerProvider = rt.LoggerProvider()
	authenticator, err := Authenticator(ctx, cfg)
	if err != nil {
		return err
	}
	readiness := rpc.Readiness{"orders": ordersConn, "reservations": reservationsConn, "bookings": bookingsConn}
	var draining atomic.Bool
	options := api.Options{Budget: cfg.RouteBudget, Authenticator: authenticator, CORSOrigins: cfg.CORSOrigins, Logger: rt.LoggerFor(reflect.TypeFor[api.Options]().PkgPath()), Ready: readiness.Check, Draining: draining.Load}
	if options.OrdersContract, err = readContract(cfg.OrdersContractPath); err != nil {
		return err
	}
	if options.ReservationsContract, err = readContract(cfg.ReservationsContractPath); err != nil {
		return err
	}
	if options.BookingsContract, err = readContract(cfg.BookingsContractPath); err != nil {
		return err
	}
	handler, err := api.NewHandler(rpc.NewOrders(ordersConn), rpc.NewReservations(reservationsConn), rpc.NewBookings(bookingsConn), ctrl, rt.Instruments(), options)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	adminListener, err := net.Listen("tcp", cfg.AdminAddr)
	if err != nil {
		_ = listener.Close()
		return err
	}
	server, admin := newServer(handler, rt), newServer(api.NewAdminHandler(options), rt)
	failed := make(chan error, 2)
	go func() { failed <- server.Serve(listener) }()
	go func() { failed <- admin.Serve(adminListener) }()
	logger.LogAttrs(ctx, slog.LevelInfo, "http listening", serverAddress(listener.Addr())...)
	logger.LogAttrs(ctx, slog.LevelInfo, "admin listening", serverAddress(adminListener.Addr())...)

	select {
	case <-ctx.Done():
		draining.Store(true)
		if cfg.DrainDelay > 0 {
			logger.InfoContext(ctx, "http draining", keyDrainDelay, cfg.DrainDelay.String())
			time.Sleep(cfg.DrainDelay)
		}
		grace, cancel := context.WithTimeout(context.WithoutCancel(ctx), observability.ShutdownGrace)
		defer cancel()
		err := server.Shutdown(grace)
		return errors.Join(err, admin.Shutdown(grace))
	case err := <-failed:
		_, _ = server.Close(), admin.Close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newServer(handler http.Handler, rt *otelboot.Runtime) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		// WHY: a nil ErrorLog sends net/http's own errors to the default logger
		// on stderr, around the redacting handler this process installs.
		ErrorLog: serverErrorLog(rt),
	}
}

// ClientOptions resolves the transport policy of both clients: TLS through the
// declared authority, or the development opt-out, logged so an audit finds it.
func ClientOptions(ctx context.Context, cfg Config, rt *otelboot.Runtime) (rpc.Options, error) {
	opts := rpc.Options{
		Clock:          obsclock.System(),
		Tracer:         rt.Tracer(),
		Instruments:    rt.Instruments(),
		LoggerProvider: rt.LoggerProvider(),
	}
	if cfg.CAFile == "" {
		// WHY: FromEnv guarantees the pair, but Run, RunWith and this function
		// are exported and a caller that builds Config by hand would otherwise
		// dial in the clear from a single empty field.
		if !cfg.GRPCInsecure {
			return rpc.Options{}, ErrInsecureNotDeclared
		}
		opts.Insecure = true
		return opts, nil
	}
	clientTLS, err := rpc.ClientTLS(cfg.CAFile, cfg.ServerName, cfg.ClientCertFile, cfg.ClientKeyFile)
	if err != nil {
		return rpc.Options{}, err
	}
	opts.TLS = clientTLS
	return opts, nil
}

var serverStatement = regexp.MustCompile(`^(?:[a-z][a-z0-9]*: )*(?:[A-Za-z][A-Za-z_., -]*)?`)

func serverErrorLog(rt *otelboot.Runtime) *log.Logger {
	return log.New(serverErrors{logger: rt.LoggerFor(reflect.TypeFor[http.Server]().PkgPath())}, "", 0)
}

type serverErrors struct{ logger *slog.Logger }

func (s serverErrors) Write(message []byte) (int, error) {
	s.logger.LogAttrs(context.Background(), slog.LevelWarn, withoutValues(string(message)))
	return len(message), nil
}

func withoutValues(message string) string {
	message = strings.TrimRight(message, "\n")
	statement := serverStatement.FindString(message)
	if len(statement) == len(message) {
		return message
	}
	return strings.TrimSpace(strings.TrimRight(statement, " ,.-") + " " + redact.Placeholder)
}

func serverAddress(addr net.Addr) []slog.Attr {
	host, port, err := net.SplitHostPort(addr.String())
	number, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil {
		return []slog.Attr{slog.String(string(semconv.ServerAddressKey), addr.String())}
	}
	return []slog.Attr{slog.String(string(semconv.ServerAddressKey), host), slog.Int(string(semconv.ServerPortKey), number)}
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
