package sqs_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	sqsPanicValue = "sqs: nil visibility output, secret access key hunter2"
	sqsChildEnv   = "DMPF_TEST_SQS_PANIC"
)

type panickingExtensions struct{ *provider.FakeSQS }

func (panickingExtensions) ChangeMessageVisibility(context.Context, *sqs.ChangeMessageVisibilityInput, ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
	panic(sqsPanicValue)
}

type panickingDeadline struct{ *clock.Fake }

func (c panickingDeadline) WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d == time.Hour {
		panic(sqsPanicValue)
	}
	return c.Fake.WithTimeout(ctx, d)
}

func startRun(t *testing.T, consumer *provider.Consumer, api provider.SQSAPI) <-chan error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx, api) }()
	return done
}

func awaitRunEnd(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not end after a goroutine of the consumer panicked: the process would run on without it")
		return nil
	}
}

func runPanickingHeartbeat(t *testing.T) (attemptAwaited bool, err error) {
	t.Helper()
	c := clock.NewFake(start)
	api := panickingExtensions{provider.NewFakeSQS()}
	returned := make(chan struct{})
	sink := newSink(func(ctx context.Context, _ handled, _ ports.Acknowledger) error {
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond)
		close(returned)
		return ctx.Err()
	})
	done := startRun(t, newConsumer(c, sink), api)

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-1", provider.EncodeBody(raw), "1")}})
	<-sink.seen
	awaitAlarm(t, c, 2)
	c.Advance(10 * time.Second)

	err = awaitRunEnd(t, done)
	select {
	case <-returned:
		return true, err
	default:
		return false, err
	}
}

func runPanickingWorker(t *testing.T) (sunk int, err error) {
	t.Helper()
	fake := clock.NewFake(start)
	api := provider.NewFakeSQS()
	sink := newSink(func(ctx context.Context, _ handled, ack ports.Acknowledger) error { return ack.Ack(ctx) })
	consumer := newConsumer(fake, sink)
	consumer.Config.Clock = panickingDeadline{fake}
	done := startRun(t, consumer, api)

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-1", provider.EncodeBody(raw), "1")}})

	err = awaitRunEnd(t, done)
	return sink.count(), err
}

func TestAPanickingHeartbeatEndsRunWithTheErrorAfterCancellingTheAttempt(t *testing.T) {
	awaited, err := runPanickingHeartbeat(t)

	if !errors.Is(err, provider.ErrPanicked) || strings.Contains(err.Error(), sqsPanicValue) {
		t.Fatalf("Run() = %v, want ErrPanicked without the panic value: cmd/main.go writes it to stderr and exits non-zero (RF-A1, ERR-20, ERR-23)", err)
	}
	if !awaited {
		t.Fatal("Run() returned with the attempt still in flight: the shutdown cancels it and waits for it")
	}
}

func TestAPanickingWorkerEndsRunWithTheError(t *testing.T) {
	sunk, err := runPanickingWorker(t)

	if !errors.Is(err, provider.ErrPanicked) || strings.Contains(err.Error(), sqsPanicValue) {
		t.Fatalf("Run() = %v, want ErrPanicked without the panic value: cmd/main.go writes it to stderr and exits non-zero (RF-A1, ERR-20, ERR-23)", err)
	}
	if sunk != 0 {
		t.Fatalf("sink handled %d messages, want 0: the panic came before it, outside the recover around Sink.Handle", sunk)
	}
}

func TestASinkPanicFailsTheAttemptWithTheSentinelAloneAndNeverItsValue(t *testing.T) {
	sink := newSink(func(context.Context, handled, ports.Acknowledger) error { panic(sqsPanicValue) })
	raw, _ := validRaw(t, "k1")

	err := newConsumer(clock.NewFake(start), sink).Attempt(context.Background(), raw)

	if !errors.Is(err, provider.ErrSinkPanicked) || err.Error() != provider.ErrSinkPanicked.Error() {
		t.Fatalf("attempt = %v, want ErrSinkPanicked alone, without the panic value (RF-A1, S-B1)", err)
	}
}

func TestRunningAPanickingConsumer(t *testing.T) {
	var err error
	switch os.Getenv(sqsChildEnv) {
	case "heartbeat":
		_, err = runPanickingHeartbeat(t)
	case "worker":
		_, err = runPanickingWorker(t)
	default:
		t.Skip("runs only as the process TestAPanickingGoroutineOfTheConsumerWritesNothingToTheStderrOfTheProcess starts")
	}
	if !errors.Is(err, provider.ErrPanicked) {
		t.Fatalf("Run() = %v, want ErrPanicked", err)
	}
}

func TestAPanickingGoroutineOfTheConsumerWritesNothingToTheStderrOfTheProcess(t *testing.T) {
	for _, scenario := range []string{"heartbeat", "worker"} {
		t.Run(scenario, func(t *testing.T) {
			stdout, stderr, err := tb.Reexec(t, "TestRunningAPanickingConsumer", sqsChildEnv+"="+scenario)

			if err != nil || stderr != "" {
				t.Fatalf("process = %v, stderr = %q, stdout = %q, want exit 0 and nothing on stderr: the error leaves only through cmd/main.go (RF-A1)", err, stderr, stdout)
			}
		})
	}
}
