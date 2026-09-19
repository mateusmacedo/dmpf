package boot_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

func telemetryUnderTest() boot.Telemetry {
	return boot.Telemetry{Service: "orders", Version: "1.2.3", Instance: "pod-1", Class: tracing.ClassWrite}
}

func TestBootHandsTheRunningRuntimeToTheWork(t *testing.T) {
	var seen *otelboot.Runtime

	err := boot.Boot(context.Background(), io.Discard, telemetryUnderTest(),
		func(_ context.Context, rt *otelboot.Runtime) error {
			seen = rt
			return nil
		})

	if err != nil {
		t.Fatalf("Boot() = %v, want nil", err)
	}
	if seen == nil {
		t.Fatal("the work was handed no runtime")
	}
	if seen.Logger() == nil || seen.Tracer() == nil {
		t.Fatal("the runtime reached the work before its pipelines were ready")
	}
}

func TestBootReturnsWhatTheWorkReturns(t *testing.T) {
	refused := errors.New("database unreachable")

	err := boot.Boot(context.Background(), io.Discard, telemetryUnderTest(),
		func(context.Context, *otelboot.Runtime) error { return refused })

	if !errors.Is(err, refused) {
		t.Fatalf("Boot() = %v, want the failure of the work", err)
	}
}

func TestBootClosesTheTelemetryEvenWhenTheWorkFails(t *testing.T) {
	var closed *otelboot.Runtime

	_ = boot.Boot(context.Background(), io.Discard, telemetryUnderTest(),
		func(_ context.Context, rt *otelboot.Runtime) error {
			closed = rt
			return errors.New("boom")
		})

	// A second shutdown is idempotent and reports the same result, so a nil
	// here proves the deferred one already ran.
	if err := closed.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil: Boot must close the pipelines on every exit", err)
	}
}

func TestBootDoesNotRunTheWorkWhenTheTelemetryRefusesToStart(t *testing.T) {
	ran := false
	telemetry := telemetryUnderTest()
	telemetry.Endpoint = "\x00 not a host"

	err := boot.Boot(context.Background(), io.Discard, telemetry,
		func(context.Context, *otelboot.Runtime) error { ran = true; return nil })

	if err == nil {
		t.Fatal("Boot() = nil with an unusable endpoint, want the startup failure")
	}
	if ran {
		t.Fatal("the work ran without telemetry; a process that lost its pipelines must refuse to start")
	}
}
