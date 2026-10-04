package app_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	purgePanicValue = "postgres: nil pool for postgres://app:hunter2@db"
	purgeChildEnv   = "DMPF_TEST_PURGE_PANIC"
)

func panickingPurge(context.Context, ports.Instant, int) (int64, error) { panic(purgePanicValue) }

func startPanickingPurge(t *testing.T) (context.Context, func() error) {
	t.Helper()
	ctx, abort := context.WithCancelCause(context.Background())
	t.Cleanup(func() { abort(nil) })
	stop, err := app.StartPurge(ctx, abort, app.PurgeConfig{Name: "outbox", Interval: time.Hour, Batch: 100}, purgeClock(1), purgeLog, panickingPurge)
	if err != nil {
		t.Fatalf("StartPurge() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = stop() })
	return ctx, stop
}

func awaitAbort(t *testing.T, ctx context.Context) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the work beside the purge was not aborted: the process would run on without its purge")
	}
}

func TestAPanickingPurgeEndsRunPurgeWithTheError(t *testing.T) {
	cfg := app.PurgeConfig{Name: "outbox", Interval: time.Hour, Batch: 100}

	err := app.RunPurge(context.Background(), cfg, purgeClock(1), purgeLog, panickingPurge)

	if !errors.Is(err, app.ErrPurgePanicked) || strings.Contains(err.Error(), purgePanicValue) {
		t.Fatalf("RunPurge() = %v, want ErrPurgePanicked without the panic value (RF-A1, ERR-20, ERR-23)", err)
	}
}

func TestAPanickingPurgeAbortsTheWorkBesideItAndStopReturnsTheError(t *testing.T) {
	ctx, stop := startPanickingPurge(t)

	awaitAbort(t, ctx)

	if cause := context.Cause(ctx); !errors.Is(cause, app.ErrPurgePanicked) {
		t.Fatalf("cause = %v, want ErrPurgePanicked: the role's loop ends because the purge panicked", cause)
	}
	if err := stop(); !errors.Is(err, app.ErrPurgePanicked) || strings.Contains(err.Error(), purgePanicValue) {
		t.Fatalf("stop() = %v, want ErrPurgePanicked without the panic value: the role returns it to cmd/main.go, which writes it to stderr (RF-A1, ERR-20)", err)
	}
}

func TestStartPurgeRefusesALoopWithoutAnAbort(t *testing.T) {
	stop, err := app.StartPurge(context.Background(), nil, app.PurgeConfig{Name: "outbox", Interval: time.Second, Batch: 100}, purgeClock(1), purgeLog, newPurgeRecorder().purge)

	if !errors.Is(err, app.ErrInvalidPurgeConfig) || stop != nil {
		t.Fatalf("StartPurge() = (stop set %v, %v), want ErrInvalidPurgeConfig: the error that ends the loop would reach no one", stop != nil, err)
	}
}

func TestRunningAPanickingPurge(t *testing.T) {
	if os.Getenv(purgeChildEnv) == "" {
		t.Skip("runs only as the process TestAPanickingPurgeWritesNothingToTheStderrOfTheProcess starts")
	}
	ctx, stop := startPanickingPurge(t)
	awaitAbort(t, ctx)
	if err := stop(); !errors.Is(err, app.ErrPurgePanicked) {
		t.Fatalf("stop() = %v, want ErrPurgePanicked", err)
	}
}

func TestAPanickingPurgeWritesNothingToTheStderrOfTheProcess(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestRunningAPanickingPurge$", "-test.count=1")
	command.Env = append(os.Environ(), purgeChildEnv+"=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr

	err := command.Run()

	if err != nil || stderr.Len() != 0 {
		t.Fatalf("process = %v, stderr = %q, stdout = %q, want exit 0 and nothing on stderr: the error leaves only through cmd/main.go (RF-A1)", err, stderr.String(), stdout.String())
	}
}
