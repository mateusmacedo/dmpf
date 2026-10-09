module github.com/mateusmacedo/dmpf/libs/backend/go/app

go 1.26.6

require (
	github.com/jackc/pgx/v5 v5.10.0
	github.com/mateusmacedo/dmpf/libs/backend/go/application v1.0.0-rc.2
	github.com/mateusmacedo/dmpf/libs/backend/go/contracts v1.0.0-rc.2
	github.com/mateusmacedo/dmpf/libs/backend/go/grpc v1.0.0-rc.2
	github.com/mateusmacedo/dmpf/libs/backend/go/observability v1.0.0-rc.2
	github.com/mateusmacedo/dmpf/libs/backend/go/ports v1.0.0-rc.2
	github.com/mateusmacedo/dmpf/libs/backend/go/postgres v1.0.0-rc.2
	github.com/mateusmacedo/dmpf/libs/backend/go/testkit v1.0.0-rc.2
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/log v1.47.0
	go.opentelemetry.io/otel/metric v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/sdk/log v1.47.0
	go.opentelemetry.io/otel/sdk/metric v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	google.golang.org/grpc v1.85.0-dev.0.20260825072537-93e31b48545e
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/domain v1.0.0-rc.2 // indirect
	github.com/mateusmacedo/dmpf/libs/backend/go/transport v1.0.0-rc.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/bridges/otelslog v0.21.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.72.0 // indirect
	go.opentelemetry.io/contrib/processors/baggagecopy v0.17.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
)
