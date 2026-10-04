package kafka_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
)

func TestEveryFailureIsRecordedUnderAnFND07CategoryOrOther(t *testing.T) {
	other := semconv.ErrorTypeOther.Value.AsString()
	cases := map[string]struct {
		err  error
		want string
	}{
		"success":                               {nil, "ok"},
		"categorized":                           {fmt.Errorf("wrapped: %w", categorized{category: "Unexpected"}), "Unexpected"},
		"context deadline":                      {fmt.Errorf("broker 10.0.0.7: %w", context.DeadlineExceeded), "DeadlineExceeded"},
		"context cancelled":                     {fmt.Errorf("broker 10.0.0.7: %w", context.Canceled), "Cancelled"},
		"sink panicked":                         {fmt.Errorf("%w: secret", kafka.ErrSinkPanicked), "Unexpected"},
		"network":                               {&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, "TransientDependency"},
		"plain error":                           {errors.New("broker 10.0.0.7: secret"), other},
		"NOT_LEADER_FOR_PARTITION":              {fmt.Errorf("produce: %w", kerr.NotLeaderForPartition), "TransientDependency"},
		"REQUEST_TIMED_OUT":                     {kerr.RequestTimedOut, "TransientDependency"},
		"UNKNOWN_TOPIC_OR_PARTITION":            {kerr.UnknownTopicOrPartition, "TransientDependency"},
		"THROTTLING_QUOTA_EXCEEDED":             {kerr.ThrottlingQuotaExceeded, "RateLimited"},
		"TOPIC_AUTHORIZATION_FAILED":            {kerr.TopicAuthorizationFailed, "Forbidden"},
		"GROUP_AUTHORIZATION_FAILED":            {kerr.GroupAuthorizationFailed, "Forbidden"},
		"CLUSTER_AUTHORIZATION_FAILED":          {kerr.ClusterAuthorizationFailed, "Forbidden"},
		"TRANSACTIONAL_ID_AUTHORIZATION_FAILED": {kerr.TransactionalIDAuthorizationFailed, "Forbidden"},
		"DELEGATION_TOKEN_AUTHORIZATION_FAILED": {kerr.DelegationTokenAuthorizationFailed, "Forbidden"},
		"SASL_AUTHENTICATION_FAILED":            {kerr.SaslAuthenticationFailed, "Unauthenticated"},
		"INVALID_RECORD":                        {kerr.InvalidRecord, "Validation"},
		"MESSAGE_TOO_LARGE":                     {kerr.MessageTooLarge, other},
		"POLICY_VIOLATION":                      {kerr.PolicyViolation, other},
		"UNKNOWN_SERVER_ERROR":                  {kerr.UnknownServerError, other},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := kafka.CategoryOf(c.err); got != c.want {
				t.Fatalf("CategoryOf(%v) = %q, want %q: FND-07 read back from the context, the network or the broker code, or %s (RF-B1)", c.err, got, c.want, other)
			}
		})
	}
}

func TestARefusedProduceIsRecordedUnderItsFND07CategoryOnTheResilienceSpan(t *testing.T) {
	fake := kafka.NewFakeClient()
	fake.ProduceErr = kerr.TopicAuthorizationFailed
	pub, recorder, _ := tracedPublisher(t, fake)
	raw, _ := validRaw(t, "k1")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := pub.Publish(ctx, "orders", raw); err == nil {
		t.Fatal("Publish() = nil, want the broker's refusal")
	}

	span := assertOnlySpan(t, recorder, "dmpf.resilience kafka")
	if got, _ := attributeOf(span, semconv.ErrorTypeKey); got.AsString() != "Forbidden" {
		t.Errorf("error.type = %q, want Forbidden, never the broker code (RF-B1)", got.AsString())
	}
}
