package otelboot

import (
	"os"
	"strconv"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const defaultAttributeValueLengthLimit = 1024

// WHY: an option of either SDK overrides the env (sdk/log@v1.47.0/setting.go:82-84,
// sdk@v1.47.0/trace/provider.go:518-522), and sdk/log reads only the LOGRECORD key
// (sdk/log@v1.47.0/provider.go:29), so the env is resolved here before the default.
func attributeValueLengthLimit(keys ...string) int {
	for _, key := range keys {
		value := os.Getenv(key)
		if value == "" {
			continue
		}
		if limit, err := strconv.Atoi(value); err == nil {
			return limit
		}
		return defaultAttributeValueLengthLimit
	}
	return defaultAttributeValueLengthLimit
}

func logAttributeValueLengthLimit() sdklog.LoggerProviderOption {
	return sdklog.WithAttributeValueLengthLimit(attributeValueLengthLimit(
		"OTEL_LOGRECORD_ATTRIBUTE_VALUE_LENGTH_LIMIT", "OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT"))
}

func spanLimits() sdktrace.TracerProviderOption {
	limits := sdktrace.NewSpanLimits()
	limits.AttributeValueLengthLimit = attributeValueLengthLimit(
		"OTEL_SPAN_ATTRIBUTE_VALUE_LENGTH_LIMIT", "OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT")
	return sdktrace.WithRawSpanLimits(limits)
}
