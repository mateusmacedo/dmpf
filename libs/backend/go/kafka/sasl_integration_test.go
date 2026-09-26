//go:build integration

package kafka_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
)

// saslBroker is a broker that demands SASL, and the principal allowed on it.
func saslBroker(t *testing.T) ([]string, kafka.SASL) {
	t.Helper()
	value := os.Getenv("KAFKA_SASL_BROKERS")
	if value == "" {
		t.Skip("KAFKA_SASL_BROKERS is not set: this test needs a broker that demands SASL")
	}
	return strings.Split(value, ","), kafka.SASL{
		Mechanism: kafka.ScramSHA256,
		Username:  os.Getenv("KAFKA_SASL_USERNAME"),
		Password:  os.Getenv("KAFKA_SASL_PASSWORD"),
	}
}

func createSASLTopic(t *testing.T, seeds []string, principal kafka.SASL, topic string) {
	t.Helper()
	cl, err := kgo.NewClient(kgo.SeedBrokers(seeds...), kgo.SASL(scram.Auth{User: principal.Username, Pass: principal.Password}.AsSha256Mechanism()))
	if err != nil {
		t.Fatalf("kgo.NewClient: %v", err)
	}
	admin := kadm.NewClient(cl)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := admin.CreateTopics(ctx, 1, 1, nil, topic); err != nil {
		t.Fatalf("CreateTopics: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = admin.DeleteTopics(ctx, topic)
		cl.Close()
	})
}

// IDN-04: the broker verifies who produces. The declared principal publishes;
// the same client with a wrong secret is refused by the broker, not by us.
func TestIntegrationSASLAuthenticatesTheProducer(t *testing.T) {
	seeds, principal := saslBroker(t)
	ch := integrationChannel(uniqueSuffix())
	createSASLTopic(t, seeds, principal, ch.Address)
	raw, _ := message(t, "k0", 0)

	cfg := integrationConfig(seeds, ch)
	cfg.SASL = &principal
	pub, err := kafka.NewPublisher(cfg, nil)
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	defer pub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := pub.Publish(ctx, ch.Name, raw); err != nil {
		t.Fatalf("Publish as the declared principal = %v, want nil", err)
	}

	forged := principal
	forged.Password = "not-the-secret"
	cfg.SASL = &forged
	intruder, err := kafka.NewPublisher(cfg, nil)
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	defer intruder.Close()
	short, cancelShort := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShort()
	if err := intruder.Publish(short, ch.Name, raw); err == nil {
		t.Fatal("Publish with a wrong secret = nil; the broker must refuse an unauthenticated producer")
	}
}
