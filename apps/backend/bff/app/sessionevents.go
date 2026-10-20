package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/api"
	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"
)

const BackchannelLogoutPath = "/backchannel-logout"

func RunSessionEvents(ctx context.Context, cfg SessionEventsConfig) error {
	return boot.Boot(ctx, SessionEventsTelemetryOf(cfg), func(ctx context.Context, rt *otelboot.Runtime) error {
		return RunSessionEventsWith(ctx, cfg, rt)
	})
}

func RunSessionEventsWith(ctx context.Context, cfg SessionEventsConfig, rt *otelboot.Runtime) error {
	logger := rt.LoggerFor(reflect.TypeFor[Config]().PkgPath())
	client, err := redis.NewClient(cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	feedConfig := cfg.Feed
	feedConfig.Clock = obsclock.System()
	feedConfig.LoggerProvider = rt.LoggerProvider()
	feedConfig.MeterProvider = rt.MeterProvider()
	feed, err := authn.NewFeed(redis.NewRevocationStore(client, feedConfig.Clock), feedConfig)
	if err != nil {
		return err
	}
	receiver, err := authn.NewBackchannelLogout(ctx, feed, cfg.Backchannel)
	if err != nil {
		return err
	}
	polling, err := authn.NewSessionPolling(feed, cfg.Polling)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return err
	}
	adminListener, err := net.Listen("tcp", cfg.AdminAddr)
	if err != nil {
		_ = listener.Close()
		return err
	}
	var draining atomic.Bool
	mux := http.NewServeMux()
	mux.Handle(BackchannelLogoutPath, receiver)
	server := newServer(mux, rt)
	admin := newServer(api.NewAdminHandler(api.Options{
		Logger:   rt.LoggerFor(reflect.TypeFor[api.Options]().PkgPath()),
		Ready:    func(context.Context) error { return nil },
		Draining: draining.Load,
	}), rt)

	pollingCtx, stopPolling := context.WithCancel(context.WithoutCancel(ctx))
	defer stopPolling()
	polled := make(chan struct{})
	go func() {
		defer close(polled)
		polling.Run(pollingCtx)
	}()
	defer func() {
		stopPolling()
		<-polled
	}()

	failed := make(chan error, 2)
	go func() { failed <- server.Serve(listener) }()
	go func() { failed <- admin.Serve(adminListener) }()
	logger.LogAttrs(ctx, slog.LevelInfo, "session events listening", serverAddress(listener.Addr())...)
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
