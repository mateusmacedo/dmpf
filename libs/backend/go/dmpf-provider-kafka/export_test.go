package dmpfkafka

import (
	"context"
	"slices"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

// The internal surface the tests in this package exercise directly.

type Cursor = cursor

func (c *Cursor) TrackRecord(r *kgo.Record) { c.Track(r) }

type Acknowledger = acknowledger

func (a *Acknowledger) Decision() int { return int(a.decision()) }

const (
	GestureUndecided = int(undecided)
	GestureAcked     = int(acked)
	GestureReleased  = int(released)
)

type Client = client

var _ Client = (*FakeClient)(nil)

// FakeClient is the in-memory client the unit tests drive: scripted fetches,
// recorded commits and produced records, and the paused set. Workers of
// different partitions call it concurrently, so every field is guarded.
type FakeClient struct {
	fetches chan kgo.Fetches

	ProduceErr error
	CommitErr  error

	mu         sync.Mutex
	commits    []*kgo.Record
	produced   []*kgo.Record
	paused     map[string][]int32
	closed     bool
	rebalances int
}

func NewFakeClient() *FakeClient {
	return &FakeClient{fetches: make(chan kgo.Fetches, 64), paused: map[string][]int32{}}
}

// Feed queues one fetch of records for a single partition.
func (f *FakeClient) Feed(topic string, partition int32, records ...*kgo.Record) {
	for _, r := range records {
		r.Topic, r.Partition = topic, partition
	}
	f.fetches <- kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: topic, Partitions: []kgo.FetchPartition{{Partition: partition, Records: records}}}}}}
}

func (f *FakeClient) PollRecords(ctx context.Context, _ int) kgo.Fetches {
	select {
	case fs := <-f.fetches:
		return fs
	case <-ctx.Done():
		return kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: "", Partitions: []kgo.FetchPartition{{Err: ctx.Err()}}}}}}
	}
}

func (f *FakeClient) CommitRecords(_ context.Context, rs ...*kgo.Record) error {
	if f.CommitErr != nil {
		return f.CommitErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits = append(f.commits, rs...)
	return nil
}

func (f *FakeClient) PauseFetchPartitions(tp map[string][]int32) map[string][]int32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	for t, ps := range tp {
		f.paused[t] = append(f.paused[t], ps...)
	}
	return f.pausedCopy()
}

func (f *FakeClient) ResumeFetchPartitions(tp map[string][]int32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for t, ps := range tp {
		kept := f.paused[t][:0:0]
		for _, p := range f.paused[t] {
			if !slices.Contains(ps, p) {
				kept = append(kept, p)
			}
		}
		if len(kept) == 0 {
			delete(f.paused, t)
			continue
		}
		f.paused[t] = kept
	}
}

func (f *FakeClient) AllowRebalance() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rebalances++
}

func (f *FakeClient) ProduceSync(_ context.Context, rs ...*kgo.Record) kgo.ProduceResults {
	f.mu.Lock()
	defer f.mu.Unlock()
	results := make(kgo.ProduceResults, 0, len(rs))
	for _, r := range rs {
		if f.ProduceErr != nil {
			results = append(results, kgo.ProduceResult{Record: r, Err: f.ProduceErr})
			continue
		}
		f.produced = append(f.produced, r)
		results = append(results, kgo.ProduceResult{Record: r})
	}
	return results
}

func (f *FakeClient) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

// Commits is a copy of the records committed so far.
func (f *FakeClient) Commits() []*kgo.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*kgo.Record(nil), f.commits...)
}

// Produced is a copy of the records produced so far.
func (f *FakeClient) Produced() []*kgo.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*kgo.Record(nil), f.produced...)
}

// Paused is a copy of the paused partitions by topic.
func (f *FakeClient) Paused() map[string][]int32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pausedCopy()
}

func (f *FakeClient) pausedCopy() map[string][]int32 {
	out := make(map[string][]int32, len(f.paused))
	for t, ps := range f.paused {
		out[t] = append([]int32(nil), ps...)
	}
	return out
}

func (f *FakeClient) Rebalances() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rebalances
}

func NewPublisherWith(cfg Config, cl client, observer Observer) (*Publisher, error) {
	return newPublisher(cfg, cl, observer)
}

func NewDLQWith(cfg Config, ch channel.Channel, cl client) (*DLQ, error) { return newDLQ(cfg, ch, cl) }

func (c *Consumer) RunWith(ctx context.Context, cl client) error { return c.run(ctx, cl) }

func (c *Consumer) Revoke(ctx context.Context, lost map[string][]int32) { c.revoked(ctx, nil, lost) }

func (c *Consumer) Assign(ctx context.Context, cl client, assigned map[string][]int32) {
	c.assign(ctx, cl, assigned)
}

func (c *Consumer) Workers() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.workers)
}

func (c *Consumer) PendingOf(topic string, partition int32) int {
	c.mu.Lock()
	w, ok := c.workers[key{topic: topic, partition: partition}]
	c.mu.Unlock()
	if !ok {
		return -1
	}
	return w.Pending()
}

func (c *Consumer) StalledAt(topic string, partition int32) bool {
	c.mu.Lock()
	w, ok := c.workers[key{topic: topic, partition: partition}]
	c.mu.Unlock()
	return ok && w.Stalled()
}
