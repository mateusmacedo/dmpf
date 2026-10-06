package otelboot

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

const platformKeyPrefix = "dmpf."

var commonLogKeys = []attribute.Key{redact.KeyErrorType, semconv.DBCollectionNameKey}

var roleLogKeys = map[string][]attribute.Key{
	"api": {
		semconv.HTTPRequestMethodKey, semconv.HTTPRouteKey, semconv.HTTPResponseStatusCodeKey,
		semconv.RPCSystemNameKey, semconv.RPCMethodKey, semconv.RPCResponseStatusCodeKey,
		semconv.ServerAddressKey, semconv.ServerPortKey,
	},
	"consumer": {
		semconv.MessagingSystemKey, semconv.MessagingOperationNameKey, semconv.MessagingDestinationNameKey,
		semconv.MessagingConsumerGroupNameKey, semconv.MessagingMessageIDKey, semconv.CloudEventsEventTypeKey,
		semconv.MessagingDestinationPartitionIDKey, semconv.MessagingKafkaOffsetKey,
	},
	"relay": {
		semconv.MessagingSystemKey, semconv.MessagingOperationNameKey, semconv.MessagingDestinationNameKey,
		semconv.MessagingMessageIDKey, semconv.CloudEventsEventTypeKey,
		semconv.MessagingDestinationPartitionIDKey, semconv.MessagingKafkaOffsetKey,
	},
}

// LogPolicy is the LOG-12 sampling of the processor. A nil Rand keeps every
// record.
type LogPolicy struct {
	Class tracing.Class
	Rates tracing.Rates
	Rand  func() float64
}

// NewLogProcessor wraps next instead of preceding it: a processor of the SDK
// cannot stop the ones registered after it (sdk/log@v1.47.0/logger.go:83-87), and a
// record sampled out by LOG-12 must not reach the batch.
func NewLogProcessor(next sdklog.Processor, policy LogPolicy) sdklog.Processor {
	return logProcessor{
		next:      next,
		sampler:   logging.NewSampler(policy.Class, policy.Rates, policy.Rand),
		allowlist: allowlists(),
		secrets:   new(secretKeys),
	}
}

type logProcessor struct {
	next      sdklog.Processor
	sampler   logging.Sampler
	allowlist map[string]map[attribute.Key]bool
	secrets   *secretKeys
}

func allowlists() map[string]map[attribute.Key]bool {
	undeclared := map[attribute.Key]bool{}
	byRole := map[string]map[attribute.Key]bool{"": undeclared}
	for role, keys := range roleLogKeys {
		allowed := map[attribute.Key]bool{}
		for _, key := range append(keys, commonLogKeys...) {
			allowed[key] = true
			undeclared[key] = true
		}
		byRole[role] = allowed
	}
	return byRole
}

func (p logProcessor) Enabled(ctx context.Context, param sdklog.EnabledParameters) bool {
	return p.next.Enabled(ctx, param)
}

func (p logProcessor) OnEmit(ctx context.Context, record *sdklog.Record) error {
	if record.EventName() != audit.EventName && !p.sampler.Allows(ctx, levelOf(record.Severity())) {
		return nil
	}

	allowed := p.allowedFor(record)
	kept := make([]attribute.KeyValue, 0, record.AttributesLen())
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		if allowed[kv.Key] || strings.HasPrefix(string(kv.Key), platformKeyPrefix) {
			kept = append(kept, p.secrets.withoutSecrets(kv))
		}
		return true
	})
	record.SetAttributes(kept...)
	return p.next.OnEmit(ctx, record)
}

func (p logProcessor) Shutdown(ctx context.Context) error   { return p.next.Shutdown(ctx) }
func (p logProcessor) ForceFlush(ctx context.Context) error { return p.next.ForceFlush(ctx) }

// allowedFor reads the role from the resource of the record and not from the
// configuration, because OTEL_RESOURCE_ATTRIBUTES may override it.
func (p logProcessor) allowedFor(record *sdklog.Record) map[attribute.Key]bool {
	role := ""
	if resource := record.Resource(); resource != nil {
		value, _ := resource.Set().Value(ProcessRoleAttribute)
		role = value.AsString()
	}
	if allowed, declared := p.allowlist[role]; declared {
		return allowed
	}
	return p.allowlist[""]
}

// levelOf inverts otelslog, which writes a slog level L as the severity L+9
// (otelslog@v0.21.0/handler.go:204).
func levelOf(severity log.Severity) slog.Level {
	return slog.Level(int(severity) - int(log.SeverityInfo))
}

const maxDecidedKeys = 1024

// secretKeys decides each key once. The keys that reach it are the allowlist and
// the dmpf.* constants of the platform, a closed set; the bound only stops a
// caller that builds keys from data from growing it.
type secretKeys struct {
	decided sync.Map
	size    atomic.Int64
}

func (s *secretKeys) isSecret(key attribute.Key) bool {
	if secret, decided := s.decided.Load(key); decided {
		return secret.(bool)
	}
	secret := redact.IsSecret(string(key))
	if s.size.Add(1) <= maxDecidedKeys {
		s.decided.Store(key, secret)
	}
	return secret
}

func (s *secretKeys) withoutSecrets(kv attribute.KeyValue) attribute.KeyValue {
	if s.isSecret(kv.Key) {
		return attribute.String(string(kv.Key), redact.Placeholder)
	}
	if kv.Value.Type() != attribute.MAP {
		return kv
	}
	nested := kv.Value.AsMap()
	clean := make([]attribute.KeyValue, len(nested))
	for i, member := range nested {
		clean[i] = s.withoutSecrets(member)
	}
	return attribute.KeyValue{Key: kv.Key, Value: attribute.MapValue(clean...)}
}
