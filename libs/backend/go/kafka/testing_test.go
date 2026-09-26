//go:build integration

package kafka_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

// brokers reads the harness variable and skips outside the CI: without a
// broker there is nothing to prove, and the CI sets it (ci.yml).
func brokers(t *testing.T) []string {
	t.Helper()
	value := os.Getenv("KAFKA_BROKERS")
	if value == "" {
		t.Skip("KAFKA_BROKERS is not set: the integration tests need a Kafka-compatible broker")
	}
	return strings.Split(value, ",")
}

// createTopics creates the channel's topics with the declared partitions and
// deletes them when the test ends, so runs never share offsets.
func createTopics(t *testing.T, seeds []string, partitions int32, topics ...string) {
	t.Helper()
	cl, err := kgo.NewClient(kgo.SeedBrokers(seeds...))
	if err != nil {
		t.Fatalf("kgo.NewClient: %v", err)
	}
	admin := kadm.NewClient(cl)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := admin.CreateTopics(ctx, partitions, 1, nil, topics...); err != nil {
		t.Fatalf("CreateTopics: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = admin.DeleteTopics(ctx, topics...)
		cl.Close()
	})
}

func uniqueSuffix() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }
