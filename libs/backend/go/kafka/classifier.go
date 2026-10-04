// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-08 (TRC-12, MET-07) que o símbolo realiza, dentro do limite de 3 linhas.

package kafka

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"github.com/twmb/franz-go/pkg/kerr"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"
)

// Classifier is the retry taxonomy of the producer: a broker error Kafka itself
// calls retriable and a network failure are worth another attempt; a
// cancellation, a deadline and any other error are not.
func Classifier(err error) retry.Retryability {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return retry.NotRetryable
	}
	var broker *kerr.Error
	if errors.As(err, &broker) {
		if broker.Retriable {
			return retry.Retryable
		}
		return retry.NotRetryable
	}
	var network net.Error
	if errors.As(err, &network) {
		return retry.Retryable
	}
	return retry.NotRetryable
}

func categoryOf(err error) string {
	if err == nil {
		return observe.CategoryOK
	}
	var categorized interface{ ErrorCategory() string }
	if errors.As(err, &categorized) {
		return categorized.ErrorCategory()
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "DeadlineExceeded"
	case errors.Is(err, context.Canceled):
		return "Cancelled"
	}
	var broker *kerr.Error
	if errors.As(err, &broker) {
		return brokerCategory(broker)
	}
	var network net.Error
	if errors.As(err, &network) {
		return categoryTransientDependency
	}
	return semconv.ErrorTypeOther.Value.AsString()
}

func errorAttr(err error) slog.Attr {
	var categorized redact.Categorized
	if err == nil || errors.As(err, &categorized) {
		return redact.Error(err)
	}
	return slog.String(redact.KeyErrorType, categoryOf(err))
}

const categoryTransientDependency = "TransientDependency"

var fnd07BrokerCodes = map[int16]string{
	kerr.ThrottlingQuotaExceeded.Code:            "RateLimited",
	kerr.TopicAuthorizationFailed.Code:           "Forbidden",
	kerr.GroupAuthorizationFailed.Code:           "Forbidden",
	kerr.ClusterAuthorizationFailed.Code:         "Forbidden",
	kerr.TransactionalIDAuthorizationFailed.Code: "Forbidden",
	kerr.DelegationTokenAuthorizationFailed.Code: "Forbidden",
	kerr.SaslAuthenticationFailed.Code:           "Unauthenticated",
	kerr.InvalidRecord.Code:                      "Validation",
}

func brokerCategory(broker *kerr.Error) string {
	if category, mapped := fnd07BrokerCodes[broker.Code]; mapped {
		return category
	}
	if broker.Retriable {
		return categoryTransientDependency
	}
	return semconv.ErrorTypeOther.Value.AsString()
}
