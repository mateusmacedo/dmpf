module github.com/mateusmacedo/dmpf/libs/backend/go/http

go 1.26.6

require (
	github.com/mateusmacedo/dmpf/libs/backend/go/observability v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/libs/backend/go/ports v1.0.0-rc.0
	github.com/mateusmacedo/dmpf/libs/backend/go/transport v1.0.0-rc.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/log v1.47.0
	go.opentelemetry.io/otel/metric v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/sdk/log v1.47.0
	go.opentelemetry.io/otel/sdk/metric v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/domain v1.0.0-rc.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	golang.org/x/sys v0.48.0 // indirect
)
