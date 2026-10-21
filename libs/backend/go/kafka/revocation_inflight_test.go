package kafka_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const waitLimit = 5 * time.Second

func revokeAsync(c *kafka.Consumer, partitions ...int32) <-chan struct{} {
	returned := make(chan struct{})
	go func() {
		c.Revoke(context.Background(), map[string][]int32{topic: partitions})
		close(returned)
	}()
	return returned
}

func receive[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(waitLimit):
		t.Fatalf("%s did not happen within %s", what, waitLimit)
	}
	var zero T
	return zero
}

func awaitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(waitLimit):
		t.Fatalf("%s did not happen within %s", what, waitLimit)
	}
}

func requireOpen(t *testing.T, ch <-chan struct{}, why string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(why)
	default:
	}
}

func ignoringHandler(entered chan<- context.Context, hold <-chan struct{}) func(context.Context, int, ports.Acknowledger) error {
	return func(ctx context.Context, _ int, _ ports.Acknowledger) error {
		entered <- ctx
		<-hold
		return nil
	}
}

func newRevocationConsumer(sink kafka.Sink) (*kafka.Consumer, *clock.Fake) {
	c := newConsumer(sink)
	clk := clock.NewFake(start)
	c.Config.Clock = clk
	return c, clk
}

func awaitGraceArmed(t *testing.T, clk *clock.Fake, before int) {
	t.Helper()
	awaitCondition(t, func() bool { return clk.Pending() == before+1 })
}

func revokeDeadlines(t *testing.T, reader interface {
	Collect(context.Context, *metricdata.ResourceMetrics) error
}) []attribute.Set {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	var sets []attribute.Set
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			if m.Name != metrics.DeadlineExceededTotal || !ok {
				continue
			}
			for _, point := range sum.DataPoints {
				for range point.Value {
					sets = append(sets, point.Attributes)
				}
			}
		}
	}
	return sets
}

func TestRevocationWaitsForAHandlerThatHonoursTheCancellation(t *testing.T) {
	cancelled := make(chan error, 1)
	letGo := make(chan struct{})
	sink := newSink()
	sink.on(0, alwaysAck)
	sink.on(1, func(ctx context.Context, _ int, ack ports.Acknowledger) error {
		<-ctx.Done()
		cancelled <- ctx.Err()
		<-letGo
		return ack.Ack(ctx)
	})
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1)...)
	c, _ := newRevocationConsumer(sink)
	done := startRunWith(t, c, fake)
	awaitCondition(t, func() bool { return sink.count() == 2 })

	returned := revokeAsync(c, 0)

	if err := receive(t, cancelled, "the cancellation of the attempt"); !errors.Is(err, context.Canceled) {
		t.Fatalf("the attempt observed %v, want cancellation (TRP-48)", err)
	}
	requireOpen(t, returned, "Revoke returned while the handler was still running")
	close(letGo)
	awaitClosed(t, returned, "the return of Revoke after the handler")

	if got := commits(fake); len(got) != 1 || got[0] != 0 {
		t.Fatalf("commits = %v, want only the prefix acknowledged before the revocation", got)
	}
	select {
	case err := <-done:
		t.Fatalf("Run() = %v after a revocation within the grace, want it still running", err)
	default:
	}
}

func TestAHandlerThatIgnoresTheCancellationEndsRunWithErrRevokeStuck(t *testing.T) {
	instruments, reader := exportedInstruments(t)
	entered := make(chan context.Context, 1)
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	sink := newSink()
	sink.on(0, alwaysAck)
	sink.on(1, ignoringHandler(entered, hold))
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0, 1)...)
	c, clk := newRevocationConsumer(sink)
	c.Config.Instruments = instruments
	done := startRunWith(t, c, fake)
	attemptCtx := receive(t, entered, "the second record reaching the sink")
	armed := clk.Pending()

	returned := revokeAsync(c, 0)

	awaitClosed(t, attemptCtx.Done(), "the cancellation of the attempt")
	awaitGraceArmed(t, clk, armed)
	requireOpen(t, returned, "Revoke returned before the revoke grace ran out")
	if c.Workers() != 0 {
		t.Fatalf("workers = %d while Revoke waits, want the partition already detached", c.Workers())
	}
	clk.Advance(c.Config.RevokeGrace)
	awaitClosed(t, returned, "the return of Revoke after the revoke grace")

	if err := awaitRunEnd(t, done); !errors.Is(err, kafka.ErrRevokeStuck) {
		t.Fatalf("Run() = %v, want ErrRevokeStuck: the stuck goroutine cannot be killed, the process must end", err)
	}
	if got := commits(fake); len(got) != 1 || got[0] != 0 {
		t.Fatalf("commits = %v, want only the prefix acknowledged before the revocation, nothing from the stuck worker", got)
	}
	sets := revokeDeadlines(t, reader)
	want := attribute.NewSet(attribute.String(metrics.KeyDependency, "kafka"), attribute.String(metrics.KeyOperation, "revoke"))
	if len(sets) != 1 || !sets[0].Equals(&want) {
		t.Fatalf("%s = %v, want one point labelled only by dependency and operation", metrics.DeadlineExceededTotal, sets)
	}
}

func TestRevocationOfTwoPartitionsCancelsBothAtOnceUnderOneGrace(t *testing.T) {
	enteredFirst := make(chan context.Context, 1)
	enteredSecond := make(chan context.Context, 1)
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	sink := newSink()
	sink.on(0, ignoringHandler(enteredFirst, hold))
	sink.on(1, ignoringHandler(enteredSecond, hold))
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)
	fake.Feed(topic, 1, recordsAt(1)...)
	c, clk := newRevocationConsumer(sink)
	done := startRunWith(t, c, fake)
	first := receive(t, enteredFirst, "partition 0 reaching the sink")
	second := receive(t, enteredSecond, "partition 1 reaching the sink")
	armed := clk.Pending()

	returned := revokeAsync(c, 0, 1)

	awaitClosed(t, first.Done(), "the cancellation of partition 0")
	awaitClosed(t, second.Done(), "the cancellation of partition 1 while partition 0 is still running")
	awaitGraceArmed(t, clk, armed)
	requireOpen(t, returned, "Revoke returned before the revoke grace ran out")
	clk.Advance(c.Config.RevokeGrace)
	awaitClosed(t, returned, "the return of Revoke after a single revoke grace for both partitions")

	if err := awaitRunEnd(t, done); !errors.Is(err, kafka.ErrRevokeStuck) {
		t.Fatalf("Run() = %v, want ErrRevokeStuck", err)
	}
	if got := commits(fake); len(got) != 0 {
		t.Fatalf("commits = %v, want none: no record was acknowledged", got)
	}
}

func TestRevocationReleasesTheIdlePartitionAndLeavesTheStuckOneUncommitted(t *testing.T) {
	entered := make(chan context.Context, 1)
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	sink := newSink()
	sink.on(0, alwaysAck)
	sink.on(1, alwaysAck)
	sink.on(2, ignoringHandler(entered, hold))
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)
	fake.Feed(topic, 1, recordsAt(1, 2)...)
	c, clk := newRevocationConsumer(sink)
	done := startRunWith(t, c, fake)
	receive(t, entered, "partition 1 reaching its stuck record")
	awaitCondition(t, func() bool { return len(fake.Commits()) == 2 })
	armed := clk.Pending()

	returned := revokeAsync(c, 0, 1)

	awaitGraceArmed(t, clk, armed)
	clk.Advance(c.Config.RevokeGrace)
	awaitClosed(t, returned, "the return of Revoke after the revoke grace")

	if err := awaitRunEnd(t, done); !errors.Is(err, kafka.ErrRevokeStuck) {
		t.Fatalf("Run() = %v, want ErrRevokeStuck", err)
	}
	byPartition := map[int32][]int64{}
	for _, r := range fake.Commits() {
		byPartition[r.Partition] = append(byPartition[r.Partition], r.Offset)
	}
	if got := byPartition[0]; len(got) != 1 || got[0] != 0 {
		t.Fatalf("commits of partition 0 = %v, want its acknowledged prefix", got)
	}
	if got := byPartition[1]; len(got) != 1 || got[0] != 1 {
		t.Fatalf("commits of partition 1 = %v, want only the commit made before the revocation: the stuck worker's gate stays closed", got)
	}
}

func TestShutdownWithAHandlerThatIgnoresTheCancellationEndsRunWithErrRevokeStuck(t *testing.T) {
	entered := make(chan context.Context, 1)
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	sink := newSink()
	sink.on(0, alwaysAck)
	sink.on(1, ignoringHandler(entered, hold))
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)
	fake.Feed(topic, 1, recordsAt(1)...)
	c, clk := newRevocationConsumer(sink)
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	done := make(chan error, 1)
	go func() { done <- c.RunWith(ctx, fake) }()
	attemptCtx := receive(t, entered, "partition 1 reaching its stuck record")
	awaitCondition(t, func() bool { return len(fake.Commits()) == 1 })
	armed := clk.Pending()

	stop()

	awaitClosed(t, attemptCtx.Done(), "the cancellation of the attempt at shutdown")
	awaitGraceArmed(t, clk, armed)
	select {
	case err := <-done:
		t.Fatalf("Run() = %v before the revoke grace ran out", err)
	default:
	}
	clk.Advance(c.Config.RevokeGrace)

	if err := awaitRunEnd(t, done); !errors.Is(err, kafka.ErrRevokeStuck) {
		t.Fatalf("Run() = %v, want ErrRevokeStuck: shutdown must not wait forever for a stuck worker", err)
	}
	for _, r := range fake.Commits() {
		if r.Partition == 1 {
			t.Fatalf("commits = %v, want nothing from the stuck partition", fake.Commits())
		}
	}
}

func TestReassigningAStuckPartitionDoesNotStartASecondWorker(t *testing.T) {
	entered := make(chan context.Context, 2)
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	sink := newSink()
	sink.on(0, ignoringHandler(entered, hold))
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)
	c, clk := newRevocationConsumer(sink)
	done := startRunWith(t, c, fake)
	receive(t, entered, "partition 0 reaching its stuck record")
	armed := clk.Pending()

	returned := revokeAsync(c, 0)
	awaitGraceArmed(t, clk, armed)
	clk.Advance(c.Config.RevokeGrace)
	awaitClosed(t, returned, "the return of Revoke after the revoke grace")
	if err := awaitRunEnd(t, done); !errors.Is(err, kafka.ErrRevokeStuck) {
		t.Fatalf("Run() = %v, want ErrRevokeStuck", err)
	}

	c.Assign(context.Background(), fake, map[string][]int32{topic: {0}})

	if c.Workers() != 0 {
		t.Fatalf("workers = %d after reassigning the stuck partition, want 0: a second worker would run the handler alongside the stuck one", c.Workers())
	}
	if paused := fake.Paused()[topic]; !slices.Contains(paused, 0) {
		t.Fatalf("paused = %v, want the stuck partition kept paused", paused)
	}
	select {
	case <-entered:
		t.Fatal("the handler ran again for the stuck partition")
	default:
	}
}
