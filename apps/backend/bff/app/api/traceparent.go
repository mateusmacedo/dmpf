package api

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/propagation"
)

const tracestateHeader = "tracestate"

type traceparentOnly struct{ propagation.TraceContext }

func (p traceparentOnly) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return p.TraceContext.Extract(ctx, withoutTracestate{carrier})
}

type withoutTracestate struct{ propagation.TextMapCarrier }

func (c withoutTracestate) Get(key string) string {
	if strings.EqualFold(key, tracestateHeader) {
		return ""
	}
	return c.TextMapCarrier.Get(key)
}
