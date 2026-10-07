module github.com/mateusmacedo/dmpf/libs/backend/go/kafka

go 1.26.6

require (
	github.com/mateusmacedo/dmpf/libs/backend/go/contracts v1.0.0-rc.1
	github.com/mateusmacedo/dmpf/libs/backend/go/observability v1.0.0-rc.1
	github.com/mateusmacedo/dmpf/libs/backend/go/ports v1.0.0-rc.1
	github.com/mateusmacedo/dmpf/libs/backend/go/transport v1.0.0-rc.1
	github.com/twmb/franz-go v1.21.6
	github.com/twmb/franz-go/pkg/kadm v1.18.0
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/log v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/sdk/log v1.47.0
	go.opentelemetry.io/otel/sdk/metric v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	google.golang.org/protobuf v1.36.12
)

require (
	go.opentelemetry.io/contrib/bridges/otelslog v0.21.0 // indirect
	go.opentelemetry.io/contrib/processors/baggagecopy v0.17.0 // indirect
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.19.1 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/domain v1.0.0-rc.1 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/testkit v1.0.0-rc.1
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.13.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)
