package kafka_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

const (
	kafkaPanicValue = "kafka: client already closed, sasl password hunter2"
	kafkaChildEnv   = "DMPF_TEST_KAFKA_PANIC"
)

type panickingPause struct{ *kafka.FakeClient }

func (panickingPause) PauseFetchPartitions(map[string][]int32) map[string][]int32 {
	panic(kafkaPanicValue)
}

type brokenClient struct{ *kafka.FakeClient }

func (brokenClient) CommitRecords(context.Context, ...*kgo.Record) error { panic(kafkaPanicValue) }

func (brokenClient) ResumeFetchPartitions(map[string][]int32) { panic(kafkaPanicValue) }

func startRunWith(t *testing.T, c *kafka.Consumer, cl kafka.Client) <-chan error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- c.RunWith(ctx, cl) }()
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

func runPanickingWorker(t *testing.T) error {
	t.Helper()
	sink := newSink()
	sink.on(0, func(context.Context, int, ports.Acknowledger) error { return nil })
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)

	return awaitRunEnd(t, startRunWith(t, newConsumer(sink), panickingPause{fake}))
}

func runPanicThatRecursAtTheShutdown(t *testing.T) error {
	t.Helper()
	sink := newSink()
	sink.on(0, func(context.Context, int, ports.Acknowledger) error { return nil })
	sink.on(1, alwaysAck)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)
	c := newConsumer(sink)
	done := startRunWith(t, c, brokenClient{fake})
	awaitCondition(t, func() bool { return c.StalledAt(topic, 0) })

	fake.Feed(topic, 1, recordsAt(1)...)

	return awaitRunEnd(t, done)
}

func TestAPanickingWorkerEndsRunWithTheError(t *testing.T) {
	err := runPanickingWorker(t)

	if !errors.Is(err, kafka.ErrPanicked) || strings.Contains(err.Error(), kafkaPanicValue) {
		t.Fatalf("Run() = %v, want ErrPanicked without the panic value: cmd/main.go writes it to stderr and exits non-zero (RF-A1, ERR-20, ERR-23)", err)
	}
}

func TestAPanicThatRecursAtTheShutdownStillEndsRunWithTheError(t *testing.T) {
	err := runPanicThatRecursAtTheShutdown(t)

	if !errors.Is(err, kafka.ErrPanicked) || strings.Contains(err.Error(), kafkaPanicValue) {
		t.Fatalf("Run() = %v, want ErrPanicked without the panic value: the shutdown lifts the pause of the stalled partition through the client that panicked in the worker", err)
	}
}

func TestASinkPanicFailsTheAttemptWithTheSentinelAloneAndNeverItsValue(t *testing.T) {
	sink := newSink()
	sink.on(0, func(context.Context, int, ports.Acknowledger) error { panic(kafkaPanicValue) })

	err := newConsumer(sink).Attempt(context.Background(), recordsAt(0)[0])

	if !errors.Is(err, kafka.ErrSinkPanicked) || err.Error() != kafka.ErrSinkPanicked.Error() {
		t.Fatalf("attempt = %v, want ErrSinkPanicked alone, without the panic value (RF-A1, S-B1)", err)
	}
}

func TestRunningAPanickingConsumer(t *testing.T) {
	var err error
	switch os.Getenv(kafkaChildEnv) {
	case "worker":
		err = runPanickingWorker(t)
	case "shutdown":
		err = runPanicThatRecursAtTheShutdown(t)
	default:
		t.Skip("runs only as the process TestAPanickingGoroutineOfTheConsumerWritesNothingToTheStderrOfTheProcess starts")
	}
	if !errors.Is(err, kafka.ErrPanicked) {
		t.Fatalf("Run() = %v, want ErrPanicked", err)
	}
}

func TestAPanickingGoroutineOfTheConsumerWritesNothingToTheStderrOfTheProcess(t *testing.T) {
	for _, scenario := range []string{"worker", "shutdown"} {
		t.Run(scenario, func(t *testing.T) {
			stdout, stderr, err := tb.Reexec(t, "TestRunningAPanickingConsumer", kafkaChildEnv+"="+scenario)

			if err != nil || stderr != "" {
				t.Fatalf("process = %v, stderr = %q, stdout = %q, want exit 0 and nothing on stderr: the error leaves only through cmd/main.go (RF-A1)", err, stderr, stdout)
			}
		})
	}
}
